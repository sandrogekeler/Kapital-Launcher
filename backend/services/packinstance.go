package services

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"kapital/backend/models"
)

// A chapter's Prism instance, created by the launcher for a fresh install
// (#22, docs/adr/0002-prism-data-root.md). Prism's own import cannot promise
// the folder name `--launch` needs (ADR-3), so the launcher writes
// <instances>/<instance id>/ itself: once, only when the folder does not
// exist, and never touches it again. What goes in is Prism's own format, as
// read in Prism 11.1.1's source:
//
//	instance.cfg                 Qt INI, written last so Prism's folder watcher
//	                             never sees a half-written instance
//	mmc-pack.json                Minecraft and the loader, from the pack's pack.toml
//	minecraft/packwiz-*.jar      the pinned jars the pre-launch command runs

// jvmPreset is a set of JVM arguments the launcher owns. A manifest names a
// preset and can never supply the arguments themselves (SECURITY_CHECKLIST S2.1).
type jvmPreset struct {
	args string
	// The oldest Minecraft whose Java runs these arguments.
	minMinecraft string
}

var jvmPresets = map[string]jvmPreset{
	// Java 21 is Minecraft's from 1.20.5. Java 24 and later ignore
	// ZGenerational with a warning, since generational ZGC became the only mode
	// (JEP 490); Java 17 refuses it and would not start.
	"zgc": {args: "-XX:+UseZGC -XX:+ZGenerational", minMinecraft: "1.20.5"},
}

// packLoaders maps pack.toml's [versions] keys to Prism's component uids
// (launcher/minecraft/Component.cpp, KNOWN_MODLOADERS).
var packLoaders = map[string]string{
	"neoforge": "net.neoforged",
	"forge":    "net.minecraftforge",
	"fabric":   "net.fabricmc.fabric-loader",
	"quilt":    "org.quiltmc.quilt-loader",
}

const (
	maxPackTOML     = 64 << 10
	packTOMLTimeout = 15 * time.Second
	// Prism's own default minimum (launcher/Application.cpp, MinMemAlloc).
	minMemMiB = 512
)

var versionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+_-]{0,63}$`)

// packVersions is what mmc-pack.json needs from a pack.toml.
type packVersions struct {
	minecraft string
	loader    string // pack.toml's key: neoforge, forge, fabric or quilt
	version   string
}

// InstanceCreator writes a chapter's Prism instance.
type InstanceCreator struct {
	// mu holds installs to one at a time: a second press, or a second chapter,
	// waits rather than racing the first for the jar cache or the folder.
	mu     sync.Mutex
	jars   *packwizJarCache
	client *http.Client
	// checkPackURL is checkURL in the app; tests swap it for a local server.
	checkPackURL func(field, raw string) error
}

// NewInstanceCreator keeps its jar cache under dataDir.
func NewInstanceCreator(dataDir string) *InstanceCreator {
	c := &InstanceCreator{jars: newPackwizJarCache(dataDir), checkPackURL: checkURL}
	c.client = &http.Client{Timeout: packTOMLTimeout, CheckRedirect: c.checkRedirect}
	return c
}

// checkRedirect holds a redirected pack.toml to the manifest's own rules.
func (c *InstanceCreator) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("too many redirects")
	}
	return c.checkPackURL("pack.toml redirect", req.URL.String())
}

// Install creates the chapter's instance in the instances folder report
// resolved (PrismService.Instances, read just before). It refuses when that
// folder is unknown or the instance is already there.
func (c *InstanceCreator) Install(ctx context.Context, chapter models.Chapter, report models.InstanceReport) error {
	if report.Dir == "" {
		return errors.New("cannot work out where Prism keeps its instances")
	}
	if report.Present[chapter.ID] {
		return fmt.Errorf("%s is already installed as %s", chapter.Name, chapter.Instance.ID)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Create(ctx, chapter, report.Dir)
}

// Create writes the chapter's instance under instancesDir. It refuses when the
// folder already exists, and removes what it wrote when it fails part-way.
func (c *InstanceCreator) Create(ctx context.Context, chapter models.Chapter, instancesDir string) error {
	id := chapter.Instance.ID
	if !prismInstanceID.MatchString(id) {
		return fmt.Errorf("instance id %q is not a plain folder name", id)
	}
	if chapter.Pack.Packwiz == nil {
		return fmt.Errorf("%s has no hosted pack to install", chapter.ID)
	}
	packURL := *chapter.Pack.Packwiz
	if err := c.checkPackURL(chapter.ID+".pack.packwiz", packURL); err != nil {
		return err
	}
	versions, err := c.fetchVersions(ctx, packURL)
	if err != nil {
		return err
	}
	if err := matchManifest(chapter.Pack, versions); err != nil {
		return fmt.Errorf("%s: %w", chapter.ID, err)
	}
	cfg, err := renderInstanceConfig(chapter, packURL)
	if err != nil {
		return err
	}
	pack, err := mmcPack(versions)
	if err != nil {
		return err
	}
	jars, err := c.jars.ensure(ctx)
	if err != nil {
		return err
	}
	files := map[string][]byte{"mmc-pack.json": pack}
	for name, path := range jars {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.Join("minecraft", name)] = data
	}
	return writeInstance(instancesDir, id, files, cfg)
}

func (c *InstanceCreator) fetchVersions(ctx context.Context, packURL string) (packVersions, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, packURL, nil)
	if err != nil {
		return packVersions{}, err
	}
	req.Header.Set("User-Agent", prismUserAgent)
	resp, err := c.client.Do(req)
	if err != nil {
		return packVersions{}, fmt.Errorf("fetch pack.toml: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only response
	if resp.StatusCode != http.StatusOK {
		return packVersions{}, fmt.Errorf("fetch pack.toml: %s", resp.Status)
	}
	return scanPackVersions(io.LimitReader(resp.Body, maxPackTOML))
}

// scanPackVersions reads the [versions] table packwiz writes: one minecraft
// key and exactly one loader, each a quoted version string. Anything else in
// that table is refused rather than guessed at.
func scanPackVersions(r io.Reader) (packVersions, error) {
	var v packVersions
	sc := bufio.NewScanner(r)
	inVersions := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inVersions = line == "[versions]"
			continue
		}
		if !inVersions {
			continue
		}
		key, raw, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		value, err := strconv.Unquote(strings.TrimSpace(raw))
		if !ok || err != nil || !versionPattern.MatchString(value) {
			return packVersions{}, fmt.Errorf("pack.toml: cannot read %q", line)
		}
		switch _, known := packLoaders[key]; {
		case key == "minecraft":
			v.minecraft = value
		case known && v.loader == "":
			v.loader, v.version = key, value
		case known:
			return packVersions{}, fmt.Errorf("pack.toml names two loaders, %s and %s", v.loader, key)
		default:
			return packVersions{}, fmt.Errorf("pack.toml: unknown version key %q", key)
		}
	}
	if err := sc.Err(); err != nil {
		return packVersions{}, fmt.Errorf("pack.toml: %w", err)
	}
	if v.minecraft == "" || v.loader == "" {
		return packVersions{}, errors.New("pack.toml names no minecraft version or no loader")
	}
	return v, nil
}

// matchManifest refuses a pack whose versions disagree with what the
// launcher shows for the chapter, since one of the two is then wrong.
func matchManifest(pack models.Pack, v packVersions) error {
	if pack.Minecraft != v.minecraft {
		return fmt.Errorf("the manifest says Minecraft %s, the pack %s", pack.Minecraft, v.minecraft)
	}
	if !strings.EqualFold(pack.Loader, v.loader) {
		return fmt.Errorf("the manifest says %s, the pack %s", pack.Loader, v.loader)
	}
	return nil
}

func mmcPack(v packVersions) ([]byte, error) {
	type component struct {
		UID       string `json:"uid"`
		Version   string `json:"version"`
		Important bool   `json:"important,omitempty"`
	}
	// The shape Prism's own Modrinth import builds: Minecraft marked important,
	// then the loader. Prism adds the loader's dependencies itself.
	data, err := json.MarshalIndent(struct {
		FormatVersion int         `json:"formatVersion"`
		Components    []component `json:"components"`
	}{1, []component{
		{UID: "net.minecraft", Version: v.minecraft, Important: true},
		{UID: packLoaders[v.loader], Version: v.version},
	}}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// preLaunchCommand is the one command a chapter's instance runs. Prism
// substitutes the $INST_ variables and then splits the string on spaces
// outside double quotes, with no shell (launch/steps/PreLaunchCommand.cpp),
// so the quotes keep a path with spaces in one argument. The URL passed
// commandSafeURL in the manifest check and carries no quote, space or $.
func preLaunchCommand(packURL string) string {
	return `"$INST_JAVA" -jar "$INST_MC_DIR/packwiz-installer-bootstrap.jar" ` +
		`--bootstrap-no-update --bootstrap-main-jar "$INST_MC_DIR/packwiz-installer.jar" ` + packURL
}

// renderInstanceConfig renders instance.cfg in the format Prism writes (QSettings
// INI, ConfigVersion 1.3). Every string is quoted and escaped the way Qt
// reads it back.
func renderInstanceConfig(chapter models.Chapter, packURL string) ([]byte, error) {
	if !commandSafeURL.MatchString(packURL) {
		return nil, fmt.Errorf("%s: pack URL %q cannot go on a command line", chapter.ID, packURL)
	}
	lines := [][2]string{
		{"ConfigVersion", "1.3"},
		{"InstanceType", "OneSix"},
		{"name", qtString(chapter.Name)},
		{"OverrideCommands", "true"},
		{"PreLaunchCommand", qtString(preLaunchCommand(packURL))},
	}
	if chapter.Pack.JVM != nil {
		preset, ok := jvmPresets[*chapter.Pack.JVM]
		if !ok {
			return nil, fmt.Errorf("%s: unknown JVM preset %q", chapter.ID, *chapter.Pack.JVM)
		}
		lines = append(lines, [2]string{"OverrideJavaArgs", "true"}, [2]string{"JvmArgs", qtString(preset.args)})
	}
	if gb := chapter.Pack.MemoryGB; gb != nil {
		// The minimum is written rather than left to the player's global
		// setting, which an overridden instance would otherwise inherit and
		// which a player may have raised. The heap grows to the maximum as
		// the game needs it, so a low start runs on any machine.
		lines = append(lines,
			[2]string{"OverrideMemory", "true"},
			[2]string{"MinMemAlloc", strconv.Itoa(minMemMiB)},
			[2]string{"MaxMemAlloc", strconv.Itoa(*gb * 1024)})
	}
	var b strings.Builder
	b.WriteString("[General]\n")
	for _, kv := range lines {
		b.WriteString(kv[0] + "=" + kv[1] + "\n")
	}
	return []byte(b.String()), nil
}

// qtString quotes a value for QSettings' INI reader: inside double quotes a
// backslash and a quote are the only characters that need escaping. A name
// with a control character never gets this far (ValidateManifest).
func qtString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// writeInstance creates <instancesDir>/<id>/, refusing one that exists, puts
// files in it and instance.cfg last. On any failure it removes the folder it
// created, and only that.
func writeInstance(instancesDir, id string, files map[string][]byte, cfg []byte) error {
	if err := os.MkdirAll(instancesDir, 0o755); err != nil {
		return err
	}
	dir := filepath.Join(instancesDir, id)
	if err := os.Mkdir(dir, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("instance %s already exists; the launcher never overwrites one", id)
		}
		return err
	}
	err := func() error {
		for name, data := range files {
			path := filepath.Join(dir, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				return err
			}
		}
		return writeFileAtomic(filepath.Join(dir, instanceConfig), cfg, 0o644)
	}()
	if err != nil {
		removeQuietly(dir)
		return fmt.Errorf("write instance %s: %w", id, err)
	}
	return nil
}

// minecraftAtLeast compares dotted release versions ("1.21.1" against
// "1.20.5"). Anything that is not one, a snapshot or a placeholder, is not.
func minecraftAtLeast(v, floor string) bool {
	parse := func(s string) ([]int, bool) {
		var out []int
		for _, part := range strings.Split(s, ".") {
			n, err := strconv.Atoi(part)
			if err != nil {
				return nil, false
			}
			out = append(out, n)
		}
		return out, true
	}
	a, okA := parse(v)
	b, okB := parse(floor)
	if !okA || !okB {
		return false
	}
	for i := 0; i < max(len(a), len(b)); i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return x > y
		}
	}
	return true
}
