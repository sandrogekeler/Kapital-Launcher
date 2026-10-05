package main

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// prismProbe stands in for the two looks at Prism a write takes, and records the
// order they were made in: the launcher's own Prism is closed before any other is
// looked for, and both before the file is touched.
type prismProbe struct {
	calls []string
	// ownGone is what closing the launcher's own Prism achieved; other and err
	// are the answer of the look for another Prism.
	ownGone bool
	other   bool
	err     error
	// exes is the executable each look was given.
	exes []string
}

func (p *prismProbe) install(app *App) {
	app.closeOwnPrism = func(chapterID string) bool {
		p.calls = append(p.calls, "close own "+chapterID)
		return p.ownGone
	}
	app.prismOpenElsewhere = func(chapterID, exe string) (bool, error) {
		p.calls = append(p.calls, "look "+chapterID)
		p.exes = append(p.exes, exe)
		return p.other, p.err
	}
}

const openPrismRefusal = "Prism is open; close it first, or the change is lost when Prism saves"

func TestASettingsSaveIsRefusedWhileAnotherPrismIsOpenAndLeavesTheFileAlone(t *testing.T) {
	app, cfg := packApp(t, commandLine("https://example.com/pack.toml"), nil)
	app.engine.Executable = filepath.Join(t.TempDir(), "prismlauncher.exe")
	probe := &prismProbe{ownGone: true, other: true}
	probe.install(app)
	before := readFile(t, cfg)

	if _, err := app.SaveChapterSettings("frangfurd", anySave); err == nil || err.Error() != openPrismRefusal {
		t.Fatalf("got %v", err)
	}
	if readFile(t, cfg) != before {
		t.Fatal("the file changed")
	}
	if got := strings.Join(probe.calls, ","); got != "close own frangfurd,look frangfurd" {
		t.Fatalf("the launcher's own Prism is closed first: %s", got)
	}
	if probe.exes[0] != app.engine.Executable {
		t.Fatalf("looked for %q, the engine's executable is %q", probe.exes[0], app.engine.Executable)
	}

	// Prism closed: the write goes through.
	probe.other = false
	saved, err := app.SaveChapterSettings("frangfurd", anySave)
	if err != nil || saved.Settings.MaxMemoryMB != 4096 {
		t.Fatalf("no Prism open: %v %+v", err, saved)
	}
}

// The launcher's own Prism that will not close is the player's to close, said in
// its own words, and no other look is made.
func TestASettingsSaveIsRefusedWhenTheLaunchersOwnPrismWillNotClose(t *testing.T) {
	app, cfg := packApp(t, commandLine("https://example.com/pack.toml"), nil)
	probe := &prismProbe{ownGone: false}
	probe.install(app)
	before := readFile(t, cfg)

	_, err := app.SaveChapterSettings("frangfurd", anySave)
	want := "The Prism opened for Frangfurd did not close when asked; close it first, or the change is lost when Prism saves"
	if err == nil || err.Error() != want {
		t.Fatalf("got %v", err)
	}
	if readFile(t, cfg) != before {
		t.Fatal("the file changed")
	}
	if got := strings.Join(probe.calls, ","); got != "close own frangfurd" {
		t.Fatalf("%s", got)
	}
}

func TestASettingsSaveGoesAheadWhenProcessesCannotBeListed(t *testing.T) {
	app, _ := packApp(t, commandLine("https://example.com/pack.toml"), nil)
	(&prismProbe{ownGone: true, err: errors.New("no snapshot")}).install(app)
	if _, err := app.SaveChapterSettings("frangfurd", anySave); err != nil {
		t.Fatalf("an OS that cannot say is not a refusal: %v", err)
	}
}

// A game the tracker or the log says is running is refused by its own guard,
// before Prism is looked at, as it was.
func TestTheGameGuardComesBeforeThePrismGuard(t *testing.T) {
	app, cfg := packApp(t, commandLine("https://example.com/pack.toml"), nil)
	probe := &prismProbe{ownGone: true, other: true}
	probe.install(app)
	trackFrangfurd(t, app, cfg)
	if _, err := app.SaveChapterSettings("frangfurd", anySave); err == nil || err.Error() != trackerRefusal {
		t.Fatalf("got %v", err)
	}
	if len(probe.calls) != 0 {
		t.Fatalf("Prism was not looked at: %v", probe.calls)
	}
}

func TestSetPackSourceIsRefusedWhileAnotherPrismIsOpenAndLeavesTheFileAlone(t *testing.T) {
	app, cfg := packApp(t, commandLine("https://example.com/pack.toml"), map[string]string{"frangfurd": devPackURL})
	probe := &prismProbe{ownGone: true, other: true}
	probe.install(app)
	before := readFile(t, cfg)

	if _, err := app.SetPackSource("frangfurd", "dev"); err == nil || err.Error() != openPrismRefusal {
		t.Fatalf("got %v", err)
	}
	if readFile(t, cfg) != before {
		t.Fatal("the file changed")
	}
	if got := strings.Join(probe.calls, ","); got != "close own frangfurd,look frangfurd" {
		t.Fatalf("%s", got)
	}

	probe.other = false
	if _, err := app.SetPackSource("frangfurd", "dev"); err != nil {
		t.Fatalf("no Prism open: %v", err)
	}
	if !strings.Contains(readFile(t, cfg), devPackURL) {
		t.Fatal("the switch was not written")
	}
}

func TestSetPackSourceIsRefusedWhenTheLaunchersOwnPrismWillNotClose(t *testing.T) {
	app, cfg := packApp(t, commandLine("https://example.com/pack.toml"), map[string]string{"frangfurd": devPackURL})
	(&prismProbe{ownGone: false}).install(app)
	before := readFile(t, cfg)
	if _, err := app.SetPackSource("frangfurd", "dev"); err == nil || !strings.Contains(err.Error(), "did not close when asked") {
		t.Fatalf("got %v", err)
	}
	if readFile(t, cfg) != before {
		t.Fatal("the file changed")
	}
}

// At Play the rewrite is skipped while a Prism is open, to be done at a later
// Play, and goes ahead when none is or when it cannot be told.
func TestUpdatePreLaunchIsSkippedWhilePrismIsOpen(t *testing.T) {
	var logs strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	cfg := filepath.Join(t.TempDir(), "instance.cfg")
	write := func() {
		t.Helper()
		if err := os.WriteFile(cfg, []byte("[General]\r\nname=x\r\n"+legacyCommand+"\r\niconKey=default\r\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write()
	before := readFile(t, cfg)
	a := &App{}
	probe := &prismProbe{other: true}
	probe.install(a)

	a.updatePreLaunch("frangfurd", filepath.Dir(cfg))
	if readFile(t, cfg) != before {
		t.Fatalf("written while Prism is open:\n%q", readFile(t, cfg))
	}
	if !strings.Contains(logs.String(), "Prism is open") {
		t.Fatalf("the skip says why:\n%s", logs.String())
	}
	if len(probe.calls) != 1 || probe.calls[0] != "look frangfurd" {
		t.Fatalf("the launcher's own Prism was closed by Play before this: %v", probe.calls)
	}

	// A later Play, with Prism closed.
	probe.other = false
	a.updatePreLaunch("frangfurd", filepath.Dir(cfg))
	if got := readFile(t, cfg); got == before || !strings.Contains(got, "-g https://") {
		t.Fatalf("the later Play brings it up:\n%q", got)
	}

	// Cannot tell: as before, the rewrite goes ahead.
	write()
	probe.err = errors.New("no snapshot")
	a.updatePreLaunch("frangfurd", filepath.Dir(cfg))
	if readFile(t, cfg) == before {
		t.Fatal("an OS that cannot say does not stop the upgrade")
	}
}
