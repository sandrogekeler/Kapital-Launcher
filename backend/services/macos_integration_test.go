//go:build darwin && macintegration

package services

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"kapital/backend/models"
)

// Checks on a real Mac that the unit tests cannot make (#30): the managed
// Prism installed from its real release and verified by codesign, the logging
// rules copied out of its bundle, its --version read through detection, the
// data roots Prism itself creates, and the launcher's sync copy run from outside
// its bundle. The macOS workflow (.github/workflows/macos.yml) runs them on an
// Apple Silicon and an Intel runner. They download Prism from GitHub and start
// it, so they sit behind the macintegration tag and outside `go test ./...`.
//
// Inputs, from the workflow:
//
//	KAPITAL_IT_PRISM_VERSION  the release tag, e.g. 11.1.1
//	KAPITAL_IT_PRISM_DIGEST   sha256:<hex> of the macOS zip, as GitHub lists it
//	KAPITAL_IT_PRISM_SIZE     the zip's size in bytes
//	KAPITAL_IT_APP            the built "Kapital Launcher.app" (sync copy test)
//	KAPITAL_IT_DISPOSABLE     1 on a runner: the tests may write to the home
//	                          folder (~/Applications, Prism's default root)

func itEnv(t *testing.T, name string) string {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		t.Skip(name + " is not set")
	}
	return v
}

func itDisposable(t *testing.T) {
	t.Helper()
	if os.Getenv("KAPITAL_IT_DISPOSABLE") != "1" {
		t.Skip("writes to the home folder; only on a disposable runner")
	}
}

// waitForFile polls for a path to exist, the way a person would watch Finder.
func waitForFile(path string, limit time.Duration) bool {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// startAndStop starts Prism with args, waits until want exists, and ends it
// with SIGTERM, as Stop does. It reports whether want appeared.
func startAndStop(t *testing.T, exe string, want string, args ...string) bool {
	t.Helper()
	cmd := exec.Command(exe, args...)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	seen := waitForFile(want, 90*time.Second)
	if err := terminateProcess(cmd.Process.Pid, false); err != nil {
		t.Error(err)
	}
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Error("Prism did not quit on SIGTERM within 20 s; killing it")
		if err := terminateProcess(cmd.Process.Pid, true); err != nil {
			t.Error(err)
		}
		<-done
	}
	return seen
}

func TestMacManagedPrism(t *testing.T) {
	version := itEnv(t, "KAPITAL_IT_PRISM_VERSION")
	digest := itEnv(t, "KAPITAL_IT_PRISM_DIGEST")
	size, err := strconv.ParseInt(itEnv(t, "KAPITAL_IT_PRISM_SIZE"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	asset, err := portableAsset(runtime.GOOS, runtime.GOARCH, version)
	if err != nil {
		t.Fatal(err)
	}
	// The real data dir has a space in it (~/Library/Application Support).
	dataDir := filepath.Join(t.TempDir(), "Application Support", "KapitalLauncher")
	m := NewManagedPrism(dataDir, runtime.GOOS, runtime.GOARCH)
	rel := models.PrismRelease{
		Version: version, Asset: asset, URL: prismDownloadBase + version + "/" + asset,
		Size: size, Digest: strings.ToLower(digest),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var phases []string
	err = m.Install(ctx, rel, func(p models.PrismInstallProgress) {
		if len(phases) == 0 || phases[len(phases)-1] != p.Phase {
			phases = append(phases, p.Phase)
		}
	})
	if err != nil {
		t.Fatalf("install: %v (phases %v)", err, phases)
	}
	t.Logf("install phases: %v", phases)
	if !strings.Contains(strings.Join(phases, ","), "verifying") {
		t.Fatalf("codesign never ran: phases %v", phases)
	}

	got, exe, ok := m.Installed()
	if !ok || got != version {
		t.Fatalf("installed %q (ok %v), want %q", got, ok, version)
	}
	info, err := os.Stat(exe)
	if err != nil || info.Mode()&0o111 == 0 {
		t.Fatalf("executable %s: %v, mode %v", exe, err, info)
	}
	app := filepath.Dir(filepath.Dir(filepath.Dir(exe)))

	t.Run("signature", func(t *testing.T) {
		// What verifySignature ran, said again with the detail the handover's
		// open question needs: who signed it, and its Team ID for a -R
		// requirement.
		out, err := exec.Command("/usr/bin/codesign", "-dv", "--verbose=2", app).CombinedOutput()
		if err != nil {
			t.Fatalf("codesign -dv: %v: %s", err, out)
		}
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "Authority=") || strings.HasPrefix(line, "TeamIdentifier=") ||
				strings.HasPrefix(line, "Identifier=") || strings.HasPrefix(line, "Format=") ||
				strings.HasPrefix(line, "Runtime Version=") {
				t.Log(line)
			}
		}
		if !strings.Contains(string(out), "Authority=Developer ID Application") {
			t.Errorf("not Developer ID signed:\n%s", out)
		}
		if out, err := exec.Command("/usr/sbin/spctl", "--assess", "--type", "execute", "-vv", app).CombinedOutput(); err != nil {
			t.Errorf("Gatekeeper refuses the managed Prism: %v: %s", err, out)
		} else {
			t.Logf("spctl: %s", strings.TrimSpace(string(out)))
		}
	})

	t.Run("no quarantine", func(t *testing.T) {
		// The launcher downloads with Go's HTTP client, which sets no
		// quarantine attribute, so Gatekeeper has nothing to ask on first open.
		if out, err := exec.Command("/usr/bin/xattr", "-p", "com.apple.quarantine", app).CombinedOutput(); err == nil {
			t.Errorf("the bundle is quarantined: %s", out)
		}
	})

	t.Run("seeded root", func(t *testing.T) {
		for _, name := range []string{"prismlauncher.cfg", "prismlauncher_update.cfg"} {
			if _, err := os.Stat(filepath.Join(m.Root(), name)); err != nil {
				t.Error(err)
			}
		}
		rules, err := os.ReadFile(filepath.Join(m.Root(), prismLogRulesFile))
		if err != nil {
			t.Fatalf("qtlogging.ini was not copied out of Contents/Resources: %v", err)
		}
		if !strings.Contains(string(rules), "launcher.task.critical=true") {
			t.Fatalf("qtlogging.ini lacks the launcher's rule:\n%s", rules)
		}
	})

	t.Run("sparkle keys", func(t *testing.T) {
		// On macOS Prism updates through Sparkle, which reads its own keys, not
		// prismlauncher_update.cfg (ADR-11). Logged for the decision.
		plist := filepath.Join(app, "Contents", "Info.plist")
		for _, key := range []string{"SUFeedURL", "SUEnableAutomaticChecks", "SUAutomaticallyUpdate", "SUPublicEDKey"} {
			out, err := exec.Command("/usr/libexec/PlistBuddy", "-c", "Print :"+key, plist).CombinedOutput()
			if err != nil {
				t.Logf("Info.plist %s: absent", key)
				continue
			}
			t.Logf("Info.plist %s = %s", key, strings.TrimSpace(string(out)))
		}
	})

	t.Run("version through detection", func(t *testing.T) {
		p := NewPrismService("darwin")
		engine := p.Detect(ctx, models.AppSettings{PrismExecutable: ResolvePrismExecutable("darwin", app)})
		if !engine.Found || engine.Source != "settings" {
			t.Fatalf("detect from the picked bundle: %+v", engine)
		}
		if engine.Version != version {
			out, err := exec.Command(exe, "--version").CombinedOutput()
			t.Fatalf("version %q, want %q; prismlauncher --version said %q (%v)", engine.Version, version, out, err)
		}
	})

	t.Run("standard location", func(t *testing.T) {
		itDisposable(t)
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(home, "Applications", "Prism Launcher.app")
		if _, err := os.Stat(dest); err == nil {
			t.Skip(dest + " already exists")
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("/usr/bin/ditto", app, dest).CombinedOutput(); err != nil {
			t.Fatalf("ditto: %v: %s", err, out)
		}
		defer func() {
			if err := os.RemoveAll(dest); err != nil {
				t.Error(err)
			}
		}()
		engine := NewPrismService("darwin").Detect(ctx, models.AppSettings{})
		want := filepath.Join(dest, "Contents", "MacOS", "prismlauncher")
		if !engine.Found || engine.Executable != want || engine.Source != "standard-location" {
			t.Fatalf("detect in ~/Applications: %+v, want %s", engine, want)
		}
		if engine.Version != version {
			t.Fatalf("version %q, want %q", engine.Version, version)
		}
	})

	t.Run("managed root through --dir", func(t *testing.T) {
		// The launch passes the managed root with --dir; Prism's own log in
		// that root is what the tracker follows (#106).
		log := filepath.Join(m.Root(), "logs", "PrismLauncher-0.log")
		if !startAndStop(t, exe, log, "--dir", m.Root()) {
			t.Fatalf("Prism started with --dir wrote no %s", log)
		}
	})

	t.Run("default data root", func(t *testing.T) {
		itDisposable(t)
		p := NewPrismService("darwin")
		root := p.DataRoot(models.AppSettings{}, models.EngineInfo{Found: true, Executable: exe, Source: "settings"})
		if root == "" {
			t.Fatal("no default root on darwin")
		}
		if _, err := os.Stat(root); err == nil {
			t.Skip(root + " already exists")
		}
		defer func() {
			if err := os.RemoveAll(root); err != nil {
				t.Error(err)
			}
		}()
		if !startAndStop(t, exe, filepath.Join(root, "logs")) {
			entries, err := os.ReadDir(filepath.Dir(root))
			names := []string{}
			for _, e := range entries {
				names = append(names, e.Name())
			}
			t.Fatalf("Prism started with no --dir created no %s; %s holds %v (%v)", root, filepath.Dir(root), names, err)
		}
	})
}

func TestMacSyncCopyRunsOutsideTheBundle(t *testing.T) {
	app := itEnv(t, "KAPITAL_IT_APP")
	exe := filepath.Join(app, "Contents", "MacOS", "kapital-launcher")
	if _, err := os.Stat(exe); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(t.TempDir(), "Application Support", "KapitalLauncher")
	sc := NewSyncCopyFrom(dataDir, "1.0.0", func() (string, error) { return exe, nil })
	copyPath, err := sc.Ensure()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(copyPath, " ") {
		t.Fatalf("the copy's path %q should carry the space the real one does", copyPath)
	}

	// The copy keeps the signature that lets it run on Apple Silicon: Wails
	// signs the bundle ad hoc, and the Mach-O carries it.
	if out, err := exec.Command("/usr/bin/codesign", "--verify", "--verbose", copyPath).CombinedOutput(); err != nil {
		t.Errorf("codesign rejects the copy: %v: %s", err, out)
	}

	// Prism splits its pre-launch command on spaces outside double quotes; the
	// launcher's own reader must get the same path back.
	cmdLine := preLaunchCommand(copyPath, "https://example.invalid/pack.toml")
	if got, _, ok := parseSyncCommand(cmdLine); !ok || got != copyPath {
		t.Fatalf("parse %q: %q, %v", cmdLine, got, ok)
	}

	run := func(args ...string) (int, string) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, copyPath, args...)
		cmd.Env = []string{"HOME=" + os.Getenv("HOME"), "PATH=/usr/bin:/bin"}
		out, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		switch {
		case err == nil:
			return 0, string(out)
		case errors.As(err, &exit) && exit.ExitCode() >= 0:
			return exit.ExitCode(), string(out)
		default:
			// A signal (SIGKILL for a bad signature, SIGABRT from dyld) lands here.
			t.Fatalf("the copy did not run to an exit: %v\n%s", err, out)
			return 0, ""
		}
	}
	// No URL: the sync's own usage line, before anything of the app starts.
	if code, out := run(SyncFlag); code != SyncExitRefused || !strings.Contains(out, "usage:") {
		t.Fatalf("exit %d, output %q", code, out)
	}
	// A URL but none of Prism's INST_ variables: refused, still cleanly.
	if code, out := run(SyncFlag, "https://kapital-packs.alessandrogekeler.workers.dev/frangfurd/pack.toml"); code != SyncExitRefused || !strings.Contains(out, "refused") {
		t.Fatalf("exit %d, output %q", code, out)
	}
}
