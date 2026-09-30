package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"kapital/backend/models"
)

// Prism Launcher's documented command line (docs/HANDOFF.md §2, verified
// against https://prismlauncher.org/wiki/getting-started/command-line-interface/
// on 2026-09-28):
//
//	-d, --dir <path>        custom application root
//	-l, --launch <id>       launch an instance by id (its folder name)
//	-s, --server <address>  join a server on launch; only with --launch
//	-a, --profile <name>    account by profile name; only with --launch
//	-I, --import <zip|url>  import an instance
//	    --show <id>         open an instance's window
//	-v, --version           print the version
//
// Everything below builds an argument *array* for exec.Command. No shell is
// ever involved, so a crafted instance id or server address cannot become a
// second command (agent_docs/SECURITY_CHECKLIST.md, S4).

var (
	prismInstanceID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	hostLabel       = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)
	prismVersion    = regexp.MustCompile(`(\d+\.\d+(?:\.\d+)?)`)
)

// ErrPrismNotFound is returned when no Prism executable could be located.
var ErrPrismNotFound = errors.New("Prism Launcher was not found; install it from https://prismlauncher.org and sign in there")

// PrismService finds and runs the Prism Launcher install the app drives.
type PrismService struct {
	// lookPath, getenv, stat, open and run are injected so detection is
	// testable without a Prism install on the machine running the tests.
	lookPath func(file string) (string, error)
	getenv   func(key string) string
	stat     func(name string) (os.FileInfo, error)
	open     func(name string) (io.ReadCloser, error)
	run      func(ctx context.Context, exe string, args ...string) ([]byte, error)
	goos     string
	home     string
	// managed reports the launcher-managed Prism, tried after every copy
	// the player installed themselves (docs/adr/0011-getting-prism.md).
	managed func() (exe, root string, ok bool)
}

// UseManaged makes detection fall back to the launcher-managed Prism.
func (p *PrismService) UseManaged(m *ManagedPrism) {
	p.managed = func() (string, string, bool) {
		_, exe, ok := m.Installed()
		return exe, m.Root(), ok
	}
}

// NewPrismService wires the real OS.
func NewPrismService(goos string) *PrismService {
	home, err := os.UserHomeDir()
	if err != nil {
		// Detection then only reaches PATH and settings; the standard locations
		// all hang off the home dir. Recorded rather than fatal: the app must
		// still open and say Prism is missing.
		home = ""
	}
	return &PrismService{
		lookPath: exec.LookPath,
		getenv:   os.Getenv,
		stat:     os.Stat,
		open:     func(name string) (io.ReadCloser, error) { return os.Open(name) },
		run: func(ctx context.Context, exe string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, exe, args...).Output()
		},
		goos: goos,
		home: home,
	}
}

// Detect finds Prism, preferring an explicit setting, then PATH, then each
// platform's standard install location, and only then the copy the launcher
// manages for a player who has none. Detection itself never downloads or
// installs anything; getting Prism is a separate step the player approves
// (docs/adr/0011-getting-prism.md). A managed Prism always runs with its own
// data root, so Root is that root whatever the settings say.
func (p *PrismService) Detect(ctx context.Context, settings models.AppSettings) models.EngineInfo {
	info := models.EngineInfo{Root: settings.PrismRoot}
	if exe := strings.TrimSpace(settings.PrismExecutable); exe != "" {
		if p.isFile(exe) {
			info.Found, info.Executable, info.Source = true, exe, "settings"
		}
	}
	if !info.Found {
		if exe, err := p.lookPath(p.executableName()); err == nil {
			info.Found, info.Executable, info.Source = true, exe, "path"
		}
	}
	if !info.Found {
		for _, candidate := range p.standardLocations() {
			if p.isFile(candidate) {
				info.Found, info.Executable, info.Source = true, candidate, "standard-location"
				break
			}
		}
	}
	if !info.Found && p.goos == "linux" {
		if flatpak, err := p.lookPath("flatpak"); err == nil && p.flatpakInstalled(ctx, flatpak) {
			info.Found, info.Executable, info.Source = true, flatpak, "flatpak"
		}
	}
	if !info.Found && p.managed != nil {
		if exe, root, ok := p.managed(); ok {
			info.Found, info.Executable, info.Source, info.Root = true, exe, "managed", root
		}
	}
	if info.Found {
		info.Version = p.readVersion(ctx, info)
	}
	return info
}

// LaunchArgs is the pure half of Launch: it validates every field and returns
// the argument array. Kept separate so the exact command line is testable.
func LaunchArgs(req models.LaunchRequest) ([]string, error) {
	if !prismInstanceID.MatchString(req.InstanceID) {
		return nil, fmt.Errorf("instance id %q is not a plain folder name", req.InstanceID)
	}
	var args []string
	if root := strings.TrimSpace(req.Root); root != "" {
		if !filepath.IsAbs(root) {
			return nil, fmt.Errorf("prism root %q must be an absolute path", root)
		}
		args = append(args, "--dir", filepath.Clean(root))
	}
	args = append(args, "--launch", req.InstanceID)
	if req.Server != "" {
		host, port, err := ParseServerAddress(req.Server)
		if err != nil {
			return nil, err
		}
		addr := host
		if port != 0 {
			addr = host + ":" + strconv.Itoa(port)
		}
		args = append(args, "--server", addr)
	}
	if profile := strings.TrimSpace(req.Profile); profile != "" {
		if strings.HasPrefix(profile, "-") || strings.ContainsAny(profile, "\x00\n\r") {
			return nil, fmt.Errorf("profile name %q could be read as an option", profile)
		}
		args = append(args, "--profile", profile)
	}
	return args, nil
}

// Launch starts Prism with the given request and returns once the process has
// started. Prism keeps running on its own; the launcher does not wait for the
// game to exit.
func (p *PrismService) Launch(ctx context.Context, engine models.EngineInfo, req models.LaunchRequest) error {
	if !engine.Found {
		return ErrPrismNotFound
	}
	args, err := LaunchArgs(req)
	if err != nil {
		return err
	}
	exe, prefix := p.command(engine)
	cmd := exec.CommandContext(context.WithoutCancel(ctx), exe, append(prefix, args...)...)
	cmd.Stdout, cmd.Stderr = nil, nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start prism: %w", err)
	}
	// Release the process: the launcher is not its parent in any sense that
	// matters, and Wait would hold the handle for as long as the game runs.
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("release prism process: %w", err)
	}
	return nil
}

// ParseServerAddress accepts host[:port] and nothing else: no scheme, no path,
// no user, so the value can only ever reach Prism's --server. Port 0 means
// "not given".
func ParseServerAddress(addr string) (string, int, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", 0, errors.New("server address is empty")
	}
	host, portStr := addr, ""
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		host, portStr = addr[:i], addr[i+1:]
		if portStr == "" {
			return "", 0, fmt.Errorf("server address %q has a colon but no port", addr)
		}
	}
	if len(host) == 0 || len(host) > 253 {
		return "", 0, fmt.Errorf("server address %q has no usable host", addr)
	}
	for _, label := range strings.Split(host, ".") {
		if !hostLabel.MatchString(label) {
			return "", 0, fmt.Errorf("server address %q is not host[:port]", addr)
		}
	}
	port := 0
	if portStr != "" {
		n, err := strconv.Atoi(portStr)
		if err != nil || n < 1 || n > 65535 {
			return "", 0, fmt.Errorf("server address %q has an invalid port", addr)
		}
		port = n
	}
	return host, port, nil
}

// command returns the executable and any fixed leading arguments. A Flatpak
// install is run through `flatpak run <app-id>`, everything else directly.
func (p *PrismService) command(engine models.EngineInfo) (string, []string) {
	if engine.Source == "flatpak" {
		return engine.Executable, []string{"run", flatpakAppID}
	}
	return engine.Executable, nil
}

const flatpakAppID = "org.prismlauncher.PrismLauncher"

func (p *PrismService) executableName() string {
	if p.goos == "windows" {
		return "prismlauncher.exe"
	}
	return "prismlauncher"
}

// standardLocations lists where each platform's installers put Prism.
// Observed on 2026-09-29: the Windows 11 installer of Prism 11.1.0 puts it
// under %LOCALAPPDATA%\Programs\PrismLauncher. The other entries are the
// installers' documented defaults and stay [verify] until seen on a real
// install (docs/adr/0007-target-oses.md).
func (p *PrismService) standardLocations() []string {
	switch p.goos {
	case "windows":
		var out []string
		if v := p.getenv("LOCALAPPDATA"); v != "" {
			out = append(out, filepath.Join(v, "Programs", "PrismLauncher", "prismlauncher.exe"))
		}
		if v := p.getenv("ProgramFiles"); v != "" {
			out = append(out, filepath.Join(v, "PrismLauncher", "prismlauncher.exe"))
		}
		if p.home != "" {
			out = append(out, filepath.Join(p.home, "scoop", "apps", "prismlauncher", "current", "prismlauncher.exe"))
		}
		return out
	case "darwin":
		out := []string{"/Applications/Prism Launcher.app/Contents/MacOS/prismlauncher"}
		if p.home != "" {
			out = append(out, filepath.Join(p.home, "Applications", "Prism Launcher.app", "Contents", "MacOS", "prismlauncher"))
		}
		return out
	default:
		var out []string
		if p.home != "" {
			out = append(out, filepath.Join(p.home, ".local", "bin", "prismlauncher"))
		}
		return append(out, "/usr/bin/prismlauncher", "/usr/local/bin/prismlauncher")
	}
}

func (p *PrismService) isFile(path string) bool {
	info, err := p.stat(path)
	return err == nil && !info.IsDir()
}

// CheckExecutable says whether a path the player typed or picked for the Prism
// executable names a file that exists. Detection would silently fall past a
// path that does not, so the settings screen asks here first (#5). The empty
// path means "detect" and passes.
func (p *PrismService) CheckExecutable(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	info, err := p.stat(path)
	if errors.Is(err, os.ErrNotExist) {
		// The OS wording ("GetFileAttributesEx ... cannot find the path") is
		// for a log, not for the line under a field.
		return fmt.Errorf("prism executable %q was not found: %w", path, os.ErrNotExist)
	}
	if err != nil {
		return fmt.Errorf("prism executable %q: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("prism executable %q is a folder, not a program", path)
	}
	return nil
}

// ResolvePrismExecutable turns what a file picker returns into the path to
// run. On macOS the picker hands back the `.app` bundle, whose executable sits
// at Contents/MacOS/prismlauncher (checked against Prism 11.1.1's macOS zip,
// docs/adr/0011-getting-prism.md); everywhere else the pick is the program.
func ResolvePrismExecutable(goos, picked string) string {
	picked = strings.TrimSpace(picked)
	if bundle := strings.TrimRight(picked, "/"); goos == "darwin" && strings.HasSuffix(bundle, ".app") {
		// A macOS path, joined with the separator macOS uses whatever host
		// runs the tests.
		return bundle + "/Contents/MacOS/prismlauncher"
	}
	return picked
}

func (p *PrismService) flatpakInstalled(ctx context.Context, flatpak string) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := p.run(ctx, flatpak, "info", flatpakAppID)
	return err == nil
}

// readVersion asks Prism for its version and keeps only the number. A failure
// is not an error: the engine card shows "linked" without a number, and the
// launch path does not depend on it.
//
// Observed on 2026-09-29 with Prism 11.1.0 on Windows 11: the GUI build writes
// "PrismLauncher 11.1.0\r\n\r\n" to stdout through a pipe, so exec's Output
// reads it without a console. The pattern takes the number wherever it sits,
// so other builds' wording does not matter; macOS and Linux output is [verify].
func (p *PrismService) readVersion(ctx context.Context, engine models.EngineInfo) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	exe, prefix := p.command(engine)
	out, err := p.run(ctx, exe, append(prefix, "--version")...)
	if err != nil {
		return ""
	}
	if m := prismVersion.FindStringSubmatch(string(out)); m != nil {
		return m[1]
	}
	return ""
}
