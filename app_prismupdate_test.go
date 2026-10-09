package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"kapital/backend/models"
)

// installManaged makes the launcher-managed Prism look installed at version,
// as ManagedPrism lays it out on this OS, and returns its executable.
func installManaged(t *testing.T, app *App, version string) string {
	t.Helper()
	dir := filepath.Join(app.dataDir, "prism")
	appDir := filepath.Join(dir, "app-"+version)
	exe := filepath.Join(appDir, "prismlauncher.exe")
	if runtime.GOOS == "darwin" {
		exe = filepath.Join(appDir, "Prism Launcher.app", "Contents", "MacOS", "prismlauncher")
	}
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("prism"), 0o755); err != nil {
		t.Fatal(err)
	}
	record := `{"version": "` + version + `", "asset": "x.zip", "digest": "sha256:00"}`
	if err := os.WriteFile(filepath.Join(dir, "managed.json"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	return exe
}

func TestAPrismUpdateIsRefusedWhileThatPrismRuns(t *testing.T) {
	app := newTestApp(t)
	exe := installManaged(t, app, "11.1.1")
	probe := &prismProbe{other: true}
	probe.install(app)
	newer := models.PrismRelease{Version: "11.2.0"}

	if err := app.refusePrismUpdateWhileOpen(newer); !errors.Is(err, errPrismUpdateOpen) {
		t.Fatalf("an update over a running Prism: got %v", err)
	}
	if len(probe.exes) != 1 || probe.exes[0] != exe {
		t.Fatalf("looked for %v, the managed executable is %q", probe.exes, exe)
	}

	// Closed: the update goes on.
	probe.other = false
	if err := app.refusePrismUpdateWhileOpen(newer); err != nil {
		t.Fatalf("an update with no Prism open: %v", err)
	}
	// Processes that cannot be listed do not block it either.
	probe.err = errors.New("no process table")
	if err := app.refusePrismUpdateWhileOpen(newer); err != nil {
		t.Fatalf("an update where processes cannot be listed: %v", err)
	}
}

func TestAPrismInstallWithNothingToRemoveIsNotRefused(t *testing.T) {
	app := newTestApp(t)
	probe := &prismProbe{other: true}
	probe.install(app)

	// No managed copy yet: a first install deletes nothing.
	if err := app.refusePrismUpdateWhileOpen(models.PrismRelease{Version: "11.2.0"}); err != nil {
		t.Fatalf("a first install: %v", err)
	}
	// The version already in place is not reinstalled, so nothing is removed.
	installManaged(t, app, "11.2.0")
	if err := app.refusePrismUpdateWhileOpen(models.PrismRelease{Version: "11.2.0"}); err != nil {
		t.Fatalf("the installed version: %v", err)
	}
	if len(probe.calls) != 0 {
		t.Fatalf("looked for processes with nothing to remove: %v", probe.calls)
	}
}
