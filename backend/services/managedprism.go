package services

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"kapital/backend/models"
)

// A launcher-managed Prism Launcher, for a player who has none
// (docs/adr/0011-getting-prism.md). On the player's approval the launcher
// downloads Prism's official portable build from Prism's own GitHub releases,
// checks it against GitHub's digest and its code signature, and unpacks it
// into its own data dir:
//
//	<data>/prism/app-<version>/   the program, replaced whole on update
//	<data>/prism/root/            Prism's data root: instances, accounts, Java
//	<data>/prism/managed.json     which version is installed
//
// It is never bundled with the app and never modified. The launcher writes
// Prism's settings once, before its first start, so Prism's setup wizard has
// nothing to ask but the Microsoft sign-in, which stays Prism's.

// EventPrismInstall carries a models.PrismInstallProgress per install step.
const EventPrismInstall = "prism:install"

const (
	prismLatestRelease = "https://api.github.com/repos/PrismLauncher/PrismLauncher/releases/latest"
	prismDownloadBase  = "https://github.com/PrismLauncher/PrismLauncher/releases/download/"
	prismUserAgent     = "KapitalLauncher (github.com/sandrogekeler/Kapital-Launcher)"
	maxReleaseJSON     = 1 << 20   // the release document, assets and notes included
	maxPrismAsset      = 250 << 20 // today's builds are 19 to 41 MB
	maxPrismUnpacked   = 1 << 30
	maxPrismEntries    = 10000
	prismAPITimeout    = 15 * time.Second
	prismDownloadStall = 60 * time.Second // no bytes for this long ends a download
	prismDownloadLimit = 2 * time.Hour    // a backstop, not a speed requirement
	progressEvery      = 512 << 10
)

// prismDownloadHosts are the only hosts the download may be served from:
// GitHub, and the storage hosts its release downloads redirect to.
var prismDownloadHosts = []string{
	"github.com",
	"objects.githubusercontent.com",
	"release-assets.githubusercontent.com",
}

var prismTag = regexp.MustCompile(`^[0-9]+\.[0-9]+(\.[0-9]+)?$`)

// prismSettingsSeed is Prism's prismlauncher.cfg for a managed root, written
// before Prism first starts. Prism 11.1.1 shows its setup wizard when the
// language is unset, Java is neither found nor downloaded automatically, a
// theme is invalid, or no Microsoft account is signed in
// (launcher/Application.cpp, createSetupWizard). This answers all but the
// last, so the sign-in page is the only one a player sees.
const prismSettingsSeed = `[General]
Language=en_US
ApplicationTheme=dark
IconTheme=pe_colored
AutomaticJavaDownload=true
AutomaticJavaSwitch=true
UserAskedAboutAutomaticJavaDownload=true
QuitAfterGameStop=true
`

// prismUpdaterSeed turns off Prism's own update check in a managed root:
// the launcher offers Prism updates itself, with its own verification, and a
// Prism update dialog would break the headless start. Prism's external
// updater reads auto_check from prismlauncher_update.cfg in the data dir and
// defaults it to on (launcher/updater/PrismExternalUpdater.cpp, 11.1.1).
const prismUpdaterSeed = `[General]
auto_check=false
`

// managedRecord is managed.json: what is installed, for the update check.
type managedRecord struct {
	Version     string    `json:"version"`
	Asset       string    `json:"asset"`
	Digest      string    `json:"digest"`
	InstalledAt time.Time `json:"installedAt"`
}

// ManagedPrism installs and finds the launcher-managed Prism.
type ManagedPrism struct {
	dir          string
	goos, goarch string

	// Injected so the whole install is testable against a local server.
	client       *http.Client
	releaseURL   string
	downloadBase string
	allowedHosts []string
	verify       func(ctx context.Context, appDir string) error
	stall        time.Duration

	mu sync.Mutex // one install at a time

	// The last self-update Reconcile looked at (#221): the version Prism
	// reported and what checking its signature gave, so a version is
	// verified and logged once, not on every look.
	driftMu      sync.Mutex
	driftVersion string
	driftErr     error
}

// NewManagedPrism keeps the managed Prism under dataDir/prism.
func NewManagedPrism(dataDir, goos, goarch string) *ManagedPrism {
	m := &ManagedPrism{
		dir:          filepath.Join(dataDir, "prism"),
		goos:         goos,
		goarch:       goarch,
		releaseURL:   prismLatestRelease,
		downloadBase: prismDownloadBase,
		allowedHosts: prismDownloadHosts,
		verify:       func(ctx context.Context, appDir string) error { return verifySignature(ctx, goos, appDir) },
		stall:        prismDownloadStall,
	}
	m.client = &http.Client{CheckRedirect: m.checkRedirect}
	return m
}

// Root is the managed Prism's data root, passed to Prism as --dir.
func (m *ManagedPrism) Root() string { return filepath.Join(m.dir, "root") }

// Installed is the managed Prism's version and executable, if there is one
// whose executable exists.
func (m *ManagedPrism) Installed() (version, exe string, ok bool) {
	rec, err := m.record()
	if err != nil || !prismTag.MatchString(rec.Version) {
		return "", "", false
	}
	exe = m.executable(m.appDir(rec.Version))
	if info, err := os.Stat(exe); err != nil || info.IsDir() {
		return "", "", false
	}
	return rec.Version, exe, true
}

// Latest reads Prism's latest release and picks this OS's portable build.
func (m *ManagedPrism) Latest(ctx context.Context) (models.PrismRelease, error) {
	ctx, cancel := context.WithTimeout(ctx, prismAPITimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.releaseURL, nil)
	if err != nil {
		return models.PrismRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", prismUserAgent)
	resp, err := m.client.Do(req)
	if err != nil {
		return models.PrismRelease{}, fmt.Errorf("read Prism's latest release: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only response
	if resp.StatusCode != http.StatusOK {
		return models.PrismRelease{}, fmt.Errorf("read Prism's latest release: %s", resp.Status)
	}
	var doc struct {
		Tag    string `json:"tag_name"`
		Page   string `json:"html_url"`
		Assets []struct {
			Name   string `json:"name"`
			Size   int64  `json:"size"`
			URL    string `json:"browser_download_url"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxReleaseJSON)).Decode(&doc); err != nil {
		return models.PrismRelease{}, fmt.Errorf("read Prism's latest release: %w", err)
	}
	if !prismTag.MatchString(doc.Tag) {
		return models.PrismRelease{}, fmt.Errorf("Prism's latest release has an unexpected tag %q", doc.Tag)
	}
	name, err := portableAsset(m.goos, m.goarch, doc.Tag)
	if err != nil {
		return models.PrismRelease{}, err
	}
	for _, a := range doc.Assets {
		if a.Name != name {
			continue
		}
		want := m.downloadBase + doc.Tag + "/" + name
		if a.URL != want {
			return models.PrismRelease{}, fmt.Errorf("%s is offered from %q, not %q", name, a.URL, want)
		}
		if !strings.HasPrefix(a.Digest, "sha256:") || len(a.Digest) != len("sha256:")+64 {
			return models.PrismRelease{}, fmt.Errorf("%s has no SHA-256 digest; refusing an unverifiable download", name)
		}
		if a.Size <= 0 || a.Size > maxPrismAsset {
			return models.PrismRelease{}, fmt.Errorf("%s is %d bytes, outside the expected size", name, a.Size)
		}
		rel := models.PrismRelease{
			Version: doc.Tag, Asset: name, URL: a.URL, Size: a.Size,
			Digest: strings.ToLower(a.Digest), Page: doc.Page,
		}
		if installed, _, ok := m.Installed(); ok {
			rel.Installed = installed
			rel.UpdateAvailable = newerVersion(doc.Tag, installed)
		}
		return rel, nil
	}
	return models.PrismRelease{}, fmt.Errorf("Prism %s has no %s", doc.Tag, name)
}

// Reconcile corrects rel for a managed Prism that updated itself (#221).
// Prism's own updater (Sparkle on macOS, which stays on: its settings are per
// user and shared with any other Prism) can replace the bundle in place, after
// which managed.json names the version the launcher installed, not the one on
// disk. detected is what detection read from the managed executable. When it
// is a different Prism version, rel says that one is installed, an update is
// offered only for a newer release, and the bundle is checked again as an
// install checks it, since the launcher did not place what now runs. A copy
// that no longer verifies is offered the latest release as a repair, which
// installs into its own folder as any update does. The first look at each
// version is logged; managed.json is left as the install wrote it, as the
// program folder's name.
func (m *ManagedPrism) Reconcile(ctx context.Context, rel *models.PrismRelease, detected string) {
	if rel.Installed == "" || detected == rel.Installed || !prismTag.MatchString(detected) {
		return
	}
	recorded := rel.Installed
	m.driftMu.Lock()
	defer m.driftMu.Unlock()
	if m.driftVersion != detected {
		m.driftVersion = detected
		m.driftErr = m.verify(ctx, m.appDir(recorded))
		slog.Info("managed prism updated outside the launcher", "recorded", recorded, "running", detected, "verified", m.driftErr == nil)
		if m.driftErr != nil {
			slog.Warn("managed prism no longer verifies", "version", detected, "error", m.driftErr)
		}
	}
	rel.Installed = detected
	rel.UpdateAvailable = newerVersion(rel.Version, detected) || m.driftErr != nil
}

// Install downloads, verifies and unpacks rel, reporting each step. An
// existing managed Prism is replaced only once the new one has passed every
// check; Prism's data root is never touched except to seed it on first
// install.
func (m *ManagedPrism) Install(ctx context.Context, rel models.PrismRelease, progress func(models.PrismInstallProgress)) (err error) {
	if !m.mu.TryLock() {
		return errors.New("Prism is already being installed")
	}
	defer m.mu.Unlock()
	defer func() {
		if err != nil {
			progress(models.PrismInstallProgress{Phase: "failed", Error: err.Error()})
		}
	}()
	if !prismTag.MatchString(rel.Version) {
		return fmt.Errorf("refusing Prism version %q", rel.Version)
	}
	// The version already in place stays: replacing it would mean deleting
	// the program folder of a Prism that may be running.
	if installed, _, ok := m.Installed(); ok && installed == rel.Version {
		progress(models.PrismInstallProgress{Phase: "done", Received: rel.Size, Total: rel.Size})
		return nil
	}
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return err
	}

	archive, err := m.download(ctx, rel, progress)
	if err != nil {
		return err
	}
	defer removeQuietly(archive)

	progress(models.PrismInstallProgress{Phase: "unpacking"})
	partial := m.appDir(rel.Version) + ".partial"
	if err := os.RemoveAll(partial); err != nil {
		return err
	}
	if err := unzipBounded(archive, partial); err != nil {
		removeQuietly(partial)
		return fmt.Errorf("unpack Prism: %w", err)
	}
	if info, err := os.Stat(m.executable(partial)); err != nil || info.IsDir() {
		removeQuietly(partial)
		return fmt.Errorf("the download has no Prism executable at %s", m.executable(partial))
	}
	progress(models.PrismInstallProgress{Phase: "verifying"})
	if err := m.verify(ctx, partial); err != nil {
		removeQuietly(partial)
		return fmt.Errorf("Prism's signature did not verify: %w", err)
	}
	// The portable build's marker makes Prism keep its data in the program
	// folder when started without --dir, and every update replaces that
	// folder. Without it, a managed Prism opened by hand uses Prism's usual
	// data folder, which no update touches. Launches always pass --dir.
	if err := os.Remove(filepath.Join(partial, "portable.txt")); err != nil && !errors.Is(err, os.ErrNotExist) {
		removeQuietly(partial)
		return fmt.Errorf("prepare Prism: %w", err)
	}

	// A leftover folder of this version (an interrupted install that never
	// recorded it) is moved aside, never deleted in place, then removed.
	final := m.appDir(rel.Version)
	if _, err := os.Stat(final); err == nil {
		aside := final + ".old"
		removeQuietly(aside)
		if err := os.Rename(final, aside); err != nil {
			removeQuietly(partial)
			return fmt.Errorf("move the previous Prism %s aside (is it running?): %w", rel.Version, err)
		}
		defer removeQuietly(aside)
	}
	if err := os.Rename(partial, final); err != nil {
		return fmt.Errorf("place Prism: %w", err)
	}
	if err := m.seedSettings(final); err != nil {
		return err
	}
	previous, _ := m.record() //nolint:errcheck // no record on a first install; nothing to clean up then
	rec := managedRecord{Version: rel.Version, Asset: rel.Asset, Digest: rel.Digest, InstalledAt: time.Now().UTC()}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(m.dir, "managed.json"), data, 0o644); err != nil {
		return err
	}
	if previous.Version != "" && previous.Version != rel.Version && prismTag.MatchString(previous.Version) {
		removeQuietly(m.appDir(previous.Version))
	}
	slog.Info("managed prism installed", "version", rel.Version, "asset", rel.Asset)
	progress(models.PrismInstallProgress{Phase: "done", Received: rel.Size, Total: rel.Size})
	return nil
}

// download streams the asset to a temp file, checking its size and SHA-256
// as it goes, and returns the file's path once both match.
func (m *ManagedPrism) download(ctx context.Context, rel models.PrismRelease, progress func(models.PrismInstallProgress)) (string, error) {
	if rel.URL != m.downloadBase+rel.Version+"/"+rel.Asset {
		return "", fmt.Errorf("refusing to download Prism from %q", rel.URL)
	}
	want := strings.TrimPrefix(rel.Digest, "sha256:")
	if len(want) != 64 || rel.Size <= 0 || rel.Size > maxPrismAsset {
		return "", errors.New("refusing a Prism download without a digest and a size")
	}
	ctx, cancel := context.WithTimeout(ctx, prismDownloadLimit)
	defer cancel()
	// A stalled connection ends the download; a slow one does not.
	stalled := time.AfterFunc(m.stall, cancel)
	defer stalled.Stop()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rel.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", prismUserAgent)
	if err := m.checkHost(req.URL); err != nil {
		return "", err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download Prism: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only response
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download Prism: %s", resp.Status)
	}

	tmp, err := os.CreateTemp(m.dir, "download-*.zip")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	fail := func(e error) (string, error) {
		if cerr := tmp.Close(); cerr != nil && !errors.Is(cerr, os.ErrClosed) {
			e = errors.Join(e, cerr)
		}
		removeQuietly(path)
		return "", e
	}
	h := sha256.New()
	counter := &progressWriter{total: rel.Size, report: progress, alive: func() { stalled.Reset(m.stall) }}
	n, err := io.Copy(io.MultiWriter(tmp, h, counter), io.LimitReader(resp.Body, rel.Size+1))
	if err != nil {
		return fail(fmt.Errorf("download Prism: %w", err))
	}
	if n != rel.Size {
		return fail(fmt.Errorf("Prism's download is %d bytes, GitHub said %d", n, rel.Size))
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fail(fmt.Errorf("Prism's download does not match GitHub's digest (got %s)", got))
	}
	if err := tmp.Close(); err != nil {
		return fail(err)
	}
	return path, nil
}

func (m *ManagedPrism) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("too many redirects")
	}
	return m.checkHost(req.URL)
}

func (m *ManagedPrism) checkHost(u *url.URL) error {
	if u.Scheme != "https" || !slices.Contains(m.allowedHosts, u.Hostname()) {
		return fmt.Errorf("refusing to fetch Prism from %s", u.Redacted())
	}
	return nil
}

// seedSettings writes Prism's settings for a managed root, each file once:
// a player's later changes in Prism are theirs. The logging rules are the third
// file, copied from the program folder appDir by seedLogRules, which never
// fails an install.
func (m *ManagedPrism) seedSettings(appDir string) error {
	for name, body := range map[string]string{
		"prismlauncher.cfg":        prismSettingsSeed,
		"prismlauncher_update.cfg": prismUpdaterSeed,
	} {
		cfg := filepath.Join(m.Root(), name)
		if _, err := os.Stat(cfg); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := writeFileAtomic(cfg, []byte(body), 0o644); err != nil {
			return err
		}
	}
	m.seedLogRules(appDir)
	return nil
}

func (m *ManagedPrism) record() (managedRecord, error) {
	var rec managedRecord
	data, err := os.ReadFile(filepath.Join(m.dir, "managed.json"))
	if err != nil {
		return rec, err
	}
	return rec, json.Unmarshal(data, &rec)
}

func (m *ManagedPrism) appDir(version string) string {
	return filepath.Join(m.dir, "app-"+version)
}

func (m *ManagedPrism) executable(appDir string) string {
	if m.goos == "darwin" {
		return filepath.Join(appDir, "Prism Launcher.app", "Contents", "MacOS", "prismlauncher")
	}
	return filepath.Join(appDir, "prismlauncher.exe")
}

// portableAsset is the release file the launcher installs on each platform
// it supports (ADR-7): Windows on x64 or arm64, and macOS.
func portableAsset(goos, goarch, version string) (string, error) {
	switch {
	case goos == "windows" && goarch == "amd64":
		return "PrismLauncher-Windows-MSVC-Portable-" + version + ".zip", nil
	case goos == "windows" && goarch == "arm64":
		return "PrismLauncher-Windows-MSVC-arm64-Portable-" + version + ".zip", nil
	case goos == "darwin":
		return "PrismLauncher-macOS-" + version + ".zip", nil
	default:
		return "", fmt.Errorf("the launcher does not install Prism on %s/%s; install it from prismlauncher.org", goos, goarch)
	}
}

// newerVersion reports whether a is a later release than b, comparing the
// dotted numbers left to right.
func newerVersion(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := range max(len(pa), len(pb)) {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i]) //nolint:errcheck // prismTag already guarantees digits
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i]) //nolint:errcheck // prismTag already guarantees digits
		}
		if x != y {
			return x > y
		}
	}
	return false
}

// unzipBounded unpacks src into dst, refusing any entry that would land
// outside dst, a symlink pointing outside it, and archives beyond the entry
// and size bounds (agent_docs/SECURITY_CHECKLIST.md, S4.2).
//
// Every folder, file and link is made through an *os.Root on dst. The Root
// resolves each path component by component and refuses any that leaves it,
// links included, so a chain such as "a" to ".", "b" to "a/..", then a file
// "b/x" cannot write above dst, which a check on the link's text alone let
// through. That is why there is no check here that a joined path stays inside
// dst: the Root is that check, and it holds when a link is followed, which a
// lexical one cannot. What stays is what the Root does not do: it creates a
// link to any text, so a target that is absolute or climbs out is refused
// before the link exists.
func unzipBounded(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close() //nolint:errcheck // read-only archive
	if len(r.File) > maxPrismEntries {
		return fmt.Errorf("%d entries, more than %d", len(r.File), maxPrismEntries)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(dst)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck // holds no data to flush
	var written int64
	var doubles []appleDoubleEntry
	for _, f := range r.File {
		// A Mac zip's AppleDouble files are read, not written: on macOS the
		// code signature attributes they hold go back on their files once
		// every file is in place (appledouble.go).
		if isAppleDoubleEntry(f.Name) {
			if f.Mode().IsDir() {
				continue
			}
			target, ok := appleDoubleTarget(f.Name)
			if !ok || !f.Mode().IsRegular() {
				return fmt.Errorf("entry %q is not an AppleDouble file", f.Name)
			}
			native := filepath.FromSlash(target)
			if filepath.IsAbs(native) || strings.Contains(target, `\`) || filepath.VolumeName(native) != "" {
				return fmt.Errorf("entry %q has an absolute or odd path", f.Name)
			}
			if runtime.GOOS != "darwin" {
				continue
			}
			raw, err := readSmall(f, maxAppleDouble+1)
			if err != nil {
				return err
			}
			if len(raw) > maxAppleDouble {
				return fmt.Errorf("entry %q is larger than an AppleDouble file", f.Name)
			}
			attrs, err := parseAppleDoubleXattrs([]byte(raw))
			if err != nil {
				return fmt.Errorf("entry %q: %w", f.Name, err)
			}
			if written += int64(len(raw)); written > maxPrismUnpacked {
				return fmt.Errorf("unpacking %q would pass %d bytes", f.Name, int64(maxPrismUnpacked))
			}
			doubles = append(doubles, appleDoubleEntry{target: filepath.Clean(native), attrs: codeSignatureAttrs(attrs)})
			continue
		}
		native := filepath.FromSlash(f.Name)
		// A rooted name is refused on every OS: Windows would not call "/x"
		// absolute, but an archive that names one is malformed.
		if strings.HasPrefix(f.Name, "/") || filepath.IsAbs(native) || strings.Contains(f.Name, `\`) || filepath.VolumeName(native) != "" {
			return fmt.Errorf("entry %q has an absolute or odd path", f.Name)
		}
		// Cleaned so a name like "a/../../x" reaches the Root as "../x", which
		// it refuses, rather than being resolved through whatever "a" is.
		name := filepath.Clean(native)
		mode := f.Mode()
		switch {
		case mode.IsDir():
			if err := root.MkdirAll(name, 0o755); err != nil {
				return fmt.Errorf("entry %q: %w", f.Name, err)
			}
		case mode&os.ModeSymlink != 0:
			// macOS app bundles link their frameworks' current versions.
			// Windows builds have no links, and Windows resolves a target
			// such as "/etc/x" against the drive root, so links are refused
			// there outright.
			if runtime.GOOS == "windows" {
				return fmt.Errorf("entry %q is a link; Prism's Windows build has none", f.Name)
			}
			link, err := readSmall(f, 4096)
			if err != nil {
				return err
			}
			if !relativeLinkInside(name, link) {
				return fmt.Errorf("link %q points outside the install folder", f.Name)
			}
			if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
				return fmt.Errorf("entry %q: %w", f.Name, err)
			}
			if err := root.Symlink(link, name); err != nil {
				return fmt.Errorf("entry %q: %w", f.Name, err)
			}
		case mode.IsRegular():
			n, err := extractFile(root, f, name, maxPrismUnpacked-written)
			if err != nil {
				return err
			}
			written += n
		default:
			return fmt.Errorf("entry %q is not a file, folder or link", f.Name)
		}
	}
	return restoreAppleDouble(root, doubles)
}

// relativeLinkInside reports whether a link at name (relative to the install
// folder) with the given target is relative and, read as text, stays inside
// that folder. It is the early refusal for an obvious escape; the Root is what
// holds when links are chained.
func relativeLinkInside(name, link string) bool {
	if link == "" || strings.HasPrefix(link, "/") || strings.Contains(link, `\`) ||
		filepath.IsAbs(link) || filepath.VolumeName(link) != "" {
		return false
	}
	joined := filepath.Join(filepath.Dir(name), link)
	return joined != ".." && !strings.HasPrefix(joined, ".."+string(filepath.Separator))
}

func extractFile(root *os.Root, f *zip.File, name string, budget int64) (int64, error) {
	if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return 0, fmt.Errorf("entry %q: %w", f.Name, err)
	}
	in, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer in.Close() //nolint:errcheck // read-only entry
	perm := os.FileMode(0o644)
	if f.Mode()&0o111 != 0 {
		perm = 0o755
	}
	out, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return 0, fmt.Errorf("entry %q: %w", f.Name, err)
	}
	n, err := io.Copy(out, io.LimitReader(in, budget+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return n, err
	}
	if n > budget {
		return n, fmt.Errorf("unpacking %q would pass %d bytes", f.Name, int64(maxPrismUnpacked))
	}
	return n, nil
}

func readSmall(f *zip.File, limit int64) (string, error) {
	in, err := f.Open()
	if err != nil {
		return "", err
	}
	defer in.Close() //nolint:errcheck // read-only entry
	b, err := io.ReadAll(io.LimitReader(in, limit))
	return string(b), err
}

func removeQuietly(path string) {
	if err := os.RemoveAll(path); err != nil {
		slog.Warn("managed prism: clean up", "path", path, "error", err)
	}
}

// progressWriter turns bytes written into download progress events, a few
// per megabyte rather than one per read.
type progressWriter struct {
	total, n, last int64
	report         func(models.PrismInstallProgress)
	alive          func() // called on every chunk, to hold off the stall timer
}

func (p *progressWriter) Write(b []byte) (int, error) {
	p.alive()
	p.n += int64(len(b))
	if p.n-p.last >= progressEvery || p.n == p.total {
		p.last = p.n
		p.report(models.PrismInstallProgress{Phase: "downloading", Received: p.n, Total: p.total})
	}
	return len(b), nil
}
