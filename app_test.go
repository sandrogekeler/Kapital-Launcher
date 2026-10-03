package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kapital/backend/models"
	"kapital/backend/services"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	app, err := NewApp(t.TempDir(), bundledManifest, nil)
	if err != nil {
		t.Fatal(err)
	}
	return app
}

// unhost takes a chapter's published pack away, for the tests of a chapter
// that hosts none now that every chapter in the bundled manifest does.
func unhost(app *App, id string) {
	for i := range app.manifest.Chapters {
		if app.manifest.Chapters[i].ID == id {
			app.manifest.Chapters[i].Pack.Packwiz = nil
		}
	}
}

func TestBundledManifestBoots(t *testing.T) {
	app := newTestApp(t)
	m, err := app.GetManifest()
	if err != nil || len(m.Chapters) == 0 {
		t.Fatalf("%v %+v", err, m)
	}
}

func TestLaunchChapterRefusesAnUnknownChapter(t *testing.T) {
	app := newTestApp(t)
	if err := app.LaunchChapter("atlantis"); err == nil || !strings.Contains(err.Error(), "no chapter") {
		t.Fatalf("got %v", err)
	}
}

func TestStopGameRefusesAnUnknownChapterAndOneWithNoRun(t *testing.T) {
	app := newTestApp(t)
	if _, err := app.StopGame("atlantis"); err == nil || !strings.Contains(err.Error(), "no chapter") {
		t.Fatalf("got %v", err)
	}
	if _, err := app.StopGame("frangfurd"); err == nil || !strings.Contains(err.Error(), "has no game to stop") {
		t.Fatalf("got %v", err)
	}
}

// Stop answers with the chapter's state after the request, and a run that has
// nothing of the launcher's alive ends at once.
func TestStopGameEndsAFollowedRunAndReturnsItsState(t *testing.T) {
	app := newTestApp(t)
	exited := make(chan struct{})
	close(exited)
	err := app.games.Track(t.Context(), services.TrackRequest{
		ChapterID:   "frangfurd",
		InstanceDir: t.TempDir(),
		Prism:       services.PrismProcess{PID: 0, Exited: exited},
		StartedAt:   time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := app.StopGame("frangfurd")
	if err != nil || got.ChapterID != "frangfurd" || got.Phase != models.GamePhaseFailed || got.Reason != models.GameFailStopped {
		t.Fatalf("%v %+v", err, got)
	}
	if app.games.Active("frangfurd") {
		t.Fatal("Play is offered again")
	}
}

// ShowPrismConsole resolves the chapter through the manifest, and a chapter with
// no hidden console is false and no error: the button was offered for one that
// has since gone.
func TestShowPrismConsoleRefusesAnUnknownChapterAndSaysFalseWithNone(t *testing.T) {
	app := newTestApp(t)
	if shown, err := app.ShowPrismConsole("atlantis"); shown || err == nil || !strings.Contains(err.Error(), "no chapter") {
		t.Fatalf("got %v, %v", shown, err)
	}
	if shown, err := app.ShowPrismConsole("frangfurd"); shown || err != nil {
		t.Fatalf("got %v, %v", shown, err)
	}
}

// Quitting with no Prism on a console returns at once.
func TestShutdownWithNoConsoleHeldReturnsAtOnce(t *testing.T) {
	app := newTestApp(t)
	done := make(chan struct{})
	go func() {
		app.shutdown(t.Context())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not return")
	}
}

// A start that was waiting on the loading card when the player quit must not
// go on to run Prism: the process is exiting, and a Prism started now would
// outlive the launcher that is meant to follow it.
func TestLaunchChapterDoesNotStartPrismOnceTheAppIsClosing(t *testing.T) {
	app := newTestApp(t)
	root := t.TempDir()
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", PrismRoot: root}); err != nil {
		t.Fatal(err)
	}
	// An engine whose executable does not exist: were Prism run, Launch would
	// fail with "start prism", and that is what this test must never see.
	app.engine = models.EngineInfo{Found: true, Executable: filepath.Join(root, "no-such-prism"), Root: root}
	ctx, cancel := context.WithCancel(context.Background())
	app.runCtx = ctx
	cancel()

	err := app.LaunchChapter("frangfurd")
	if err == nil || !strings.Contains(err.Error(), "closing") {
		t.Fatalf("got %v", err)
	}
}

// A managed Prism installed before its log rules were seeded gets them on the
// next Play, before Prism starts (#103, #106); a Prism the player installed is
// never written to. Prism is made to fail on start, so the file existing after
// the failed launch shows the seed came first.
func TestLaunchChapterSeedsTheManagedPrismsLogRulesBeforePrismStarts(t *testing.T) {
	run := func(t *testing.T, source string) (rulesInRoot bool) {
		t.Helper()
		dataDir := t.TempDir()
		app, err := NewApp(dataDir, bundledManifest, nil)
		if err != nil {
			t.Fatal(err)
		}
		managed := services.NewManagedPrism(dataDir, "windows", "amd64")
		app.managed = managed
		appDir := filepath.Join(dataDir, "prism", "app-11.1.1")
		if err := os.MkdirAll(appDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dataDir, "prism", "managed.json"), []byte(`{"version":"11.1.1"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(appDir, "qtlogging.ini"), []byte("[Rules]\n*.debug=true\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := app.SaveSettings(models.AppSettings{Theme: "dark"}); err != nil {
			t.Fatal(err)
		}
		app.engine = models.EngineInfo{Found: true, Source: source, Executable: filepath.Join(appDir, "no-such-prism"), Root: managed.Root()}

		err = app.LaunchChapter("frangfurd")
		if err == nil || !strings.Contains(err.Error(), "start prism") {
			t.Fatalf("Prism is started and fails here: %v", err)
		}
		_, statErr := os.Stat(filepath.Join(managed.Root(), "qtlogging.ini"))
		return statErr == nil
	}
	if !run(t, "managed") {
		t.Error("a managed Prism's root has the rules by the time Prism starts")
	}
	if run(t, "standard-location") {
		t.Error("nothing is written for a Prism that is not the launcher's")
	}
}

func TestGetServerStatusRefusesAnUnknownChapterAndAnswersForOne(t *testing.T) {
	app := newTestApp(t)
	if _, err := app.GetServerStatus("atlantis"); err == nil {
		t.Fatal("unknown chapter must be refused")
	}
	// Luxemburg has no server: checked, offline, no dial attempted.
	got, err := app.GetServerStatus("luxemburg")
	if err != nil || !got.Checked || got.Online || got.ChapterID != "luxemburg" {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestSaveSettingsValidatesAndRedetects(t *testing.T) {
	app := newTestApp(t)
	if err := app.SaveSettings(models.AppSettings{Theme: "sepia"}); err == nil {
		t.Fatal("an invalid theme must be refused")
	}
	if err := app.SaveSettings(models.AppSettings{Theme: "light", LastChapter: "frangfurd"}); err != nil {
		t.Fatal(err)
	}
	got, err := app.GetSettings()
	if err != nil || got.Theme != "light" || got.LastChapter != "frangfurd" {
		t.Fatalf("%v %+v", err, got)
	}
}

// A Prism executable that changed must exist as a file when it is saved; one
// already on file may vanish without blocking the next save (#5).
func TestSaveSettingsChecksAChangedExecutableOnly(t *testing.T) {
	app := newTestApp(t)
	dir := t.TempDir()
	missing := filepath.Join(dir, "prismlauncher.exe")
	err := app.SaveSettings(models.AppSettings{Theme: "dark", PrismExecutable: missing})
	if err == nil || !strings.Contains(err.Error(), "prism executable") {
		t.Fatalf("a missing program must be refused: %v", err)
	}
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", PrismExecutable: dir}); err == nil {
		t.Fatal("a folder must be refused")
	}
	if err := os.WriteFile(missing, []byte("not really prism"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", PrismExecutable: missing}); err != nil {
		t.Fatalf("an existing file is accepted: %v", err)
	}
	if err := os.Remove(missing); err != nil {
		t.Fatal(err)
	}
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", PrismExecutable: missing, LastChapter: "frangfurd"}); err != nil {
		t.Fatalf("an unchanged executable is not checked again: %v", err)
	}
	got, err := app.GetSettings()
	if err != nil || got.LastChapter != "frangfurd" {
		t.Fatalf("%v %+v", err, got)
	}
}

// The pickers need the window; before it exists they refuse rather than hang,
// and they never save (the frontend commits the pick through SaveSettings).
func TestPickersNeedTheWindow(t *testing.T) {
	app := newTestApp(t)
	if _, err := app.ChoosePrismExecutable(); err == nil || !strings.Contains(err.Error(), "window") {
		t.Fatalf("got %v", err)
	}
	if _, err := app.ChoosePrismRoot(); err == nil || !strings.Contains(err.Error(), "window") {
		t.Fatalf("got %v", err)
	}
}

// A chapter's settings are read from and written into its own instance.cfg,
// only when the instance exists, only within the preset list and the
// machine's memory (#36).
func TestChapterSettingsRoundTripThroughTheInstance(t *testing.T) {
	app := newTestApp(t)
	root := t.TempDir()
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", PrismRoot: root}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.GetChapterSettings("frangfurd"); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("no instance yet: %v", err)
	}
	inst := filepath.Join(root, "instances", "kapital-frangfurd")
	if err := os.MkdirAll(inst, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "[General]\nname=Frangfurd\nOverrideMemory=true\nMinMemAlloc=512\nMaxMemAlloc=8192\n"
	if err := os.WriteFile(filepath.Join(inst, "instance.cfg"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := app.GetChapterSettings("frangfurd")
	if err != nil || info.Settings.MaxMemoryMB != 8192 || info.Settings.JVM != "" || info.PackMemoryMB != 8192 || info.Running {
		t.Fatalf("%v %+v", err, info)
	}
	if _, err := app.SaveChapterSettings("frangfurd", models.ChapterSettings{MaxMemoryMB: 4096, JVM: "-Xmx1g"}); err == nil {
		t.Fatal("raw arguments are refused")
	}
	saved, err := app.SaveChapterSettings("frangfurd", models.ChapterSettings{MaxMemoryMB: 4096, JVM: "zgc"})
	if err != nil || saved.Settings.MaxMemoryMB != 4096 || saved.Settings.JVM != "zgc" {
		t.Fatalf("%v %+v", err, saved)
	}
	raw, err := os.ReadFile(filepath.Join(inst, "instance.cfg"))
	if err != nil || !strings.HasPrefix(string(raw), "[General]\nname=Frangfurd\n") {
		t.Fatalf("the other lines stay: %v %q", err, raw)
	}
	if _, err := app.SaveChapterSettings("atlantis", models.ChapterSettings{MaxMemoryMB: 4096}); err == nil {
		t.Fatal("unknown chapter")
	}
}

// The folder opened is the chapter's instance folder under the resolved Prism
// root: the caller names a chapter, never a path, and a chapter whose instance
// is missing is refused before the file manager is asked (#85).
func TestOpenInstanceFolderOpensOnlyAnInstalledChaptersFolder(t *testing.T) {
	app := newTestApp(t)
	var opened []string
	app.openFolder = func(dir string) error { opened = append(opened, dir); return nil }
	root := t.TempDir()
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", PrismRoot: root}); err != nil {
		t.Fatal(err)
	}
	if err := app.OpenInstanceFolder("atlantis"); err == nil {
		t.Fatal("unknown chapter")
	}
	if err := app.OpenInstanceFolder("frangfurd"); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("no instance yet: %v", err)
	}
	inst := filepath.Join(root, "instances", "kapital-frangfurd")
	if err := os.MkdirAll(inst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inst, "instance.cfg"), []byte("[General]\nname=Frangfurd\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 0 {
		t.Fatalf("opened before the instance existed: %v", opened)
	}
	if err := app.OpenInstanceFolder("frangfurd"); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 1 || opened[0] != inst {
		t.Fatalf("opened %v, want [%s]", opened, inst)
	}
	app.openFolder = func(string) error { return errors.New("no file manager") }
	if err := app.OpenInstanceFolder("frangfurd"); err == nil || !strings.Contains(err.Error(), "Frangfurd") {
		t.Fatalf("the failure names the chapter: %v", err)
	}
}

// The bridge may only open a wiki page the app itself listed (#58).
func TestOpenWikiPageRefusesAnUnlistedURL(t *testing.T) {
	app := newTestApp(t)
	err := app.OpenWikiPage("https://kapitel-kapital.pages.dev/wiki/locations/the-obelisk")
	if err == nil || !strings.Contains(err.Error(), "not a wiki page") {
		t.Fatalf("nothing listed yet, so nothing opens: %v", err)
	}
}

func TestOpenExternalRefusesNonWebURLsBeforeTheWindowExists(t *testing.T) {
	app := newTestApp(t)
	if err := app.OpenExternal("file:///etc/passwd"); err == nil || strings.Contains(err.Error(), "window") {
		t.Fatalf("the scheme check must run before the window check: %v", err)
	}
	if err := app.OpenExternal("https://prismlauncher.org"); err == nil || !strings.Contains(err.Error(), "window") {
		t.Fatalf("with no window there is nothing to open with: %v", err)
	}
}

// Every exported method on App must return an error as its last value, so the
// frontend always has a rejection to handle (.claude/rules/ipc.md). Read from
// the source rather than by reflection, so a method with no error return is
// named in the failure.
func TestBoundMethodsReturnAnError(t *testing.T) {
	// Wails binds every exported method of App wherever it is declared, so the
	// check reads each app*.go and not app.go alone.
	files, err := filepath.Glob("app*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(src), "\n") {
			if !strings.HasPrefix(line, "func (a *App) ") {
				continue
			}
			name := strings.TrimPrefix(line, "func (a *App) ")
			name = name[:strings.Index(name, "(")]
			if name == "" || name[0] < 'A' || name[0] > 'Z' {
				continue
			}
			if !strings.HasSuffix(strings.TrimSpace(line), "error {") && !strings.HasSuffix(strings.TrimSpace(line), "error) {") {
				t.Errorf("%s (%s) does not return an error as its last value", name, file)
			}
		}
	}
}

func TestGetInstancesAnswersForEveryChapterUnderAConfiguredRoot(t *testing.T) {
	app := newTestApp(t)
	root := t.TempDir()
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", PrismRoot: root}); err != nil {
		t.Fatal(err)
	}
	inst := filepath.Join(root, "instances", "kapital-luxemburg")
	if err := os.MkdirAll(inst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inst, "instance.cfg"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := app.GetInstances()
	if err != nil || got.Root != root {
		t.Fatalf("%v %+v", err, got)
	}
	if !got.Present["luxemburg"] || got.Present["lichdenstein"] || got.Present["frangfurd"] {
		t.Fatalf("%+v", got.Present)
	}
}

func TestInstallChapterRefusesBeforeWritingAnything(t *testing.T) {
	app := newTestApp(t)
	root := t.TempDir()
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", PrismRoot: root}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.InstallChapter("atlantis"); err == nil {
		t.Error("an unknown chapter was not refused")
	}
	app.engine = models.EngineInfo{}
	if _, err := app.InstallChapter("frangfurd"); err == nil {
		t.Error("an install without Prism was not refused")
	}
	// A chapter that hosts no pack (every chapter does since kapital-packs#12,
	// so Lichdenstein's is taken away here): Install has nothing to write, and
	// nothing is fetched.
	unhost(app, "lichdenstein")
	app.engine = models.EngineInfo{Found: true, Source: "settings"}
	if _, err := app.InstallChapter("lichdenstein"); err == nil || !strings.Contains(err.Error(), "no hosted pack") {
		t.Errorf("expected the missing pack to be refused, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "instances", "kapital-lichdenstein")); !os.IsNotExist(err) {
		t.Fatalf("a refused install wrote into the Prism root: %v", err)
	}
}

func TestInstallChapterTakesALocalPackFromSettings(t *testing.T) {
	app := newTestApp(t)
	root := t.TempDir()
	// Port 9 (discard) on loopback: nothing answers, so the install stops at
	// fetching pack.toml, past the check that the manifest hosts no pack.
	settings := models.AppSettings{Theme: "dark", PrismRoot: root, PackOverrides: map[string]string{"frangfurd": "http://127.0.0.1:9/pack.toml"}}
	if err := app.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	app.engine = models.EngineInfo{Found: true, Source: "settings"}
	_, err := app.InstallChapter("frangfurd")
	if err == nil || !strings.Contains(err.Error(), "fetch pack.toml") {
		t.Fatalf("expected the local pack to be fetched, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "instances", "kapital-frangfurd")); !os.IsNotExist(err) {
		t.Fatalf("a failed install left a folder: %v", err)
	}
}

// A start the launcher follows holds Play, installs and settings saves for its
// chapter, and GetGameStates reports it beside the chapters never launched (#44).
func TestAChapterBeingFollowedIsRunningForEveryGuard(t *testing.T) {
	app := newTestApp(t)
	root := t.TempDir()
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", PrismRoot: root}); err != nil {
		t.Fatal(err)
	}
	inst := filepath.Join(root, "instances", "kapital-frangfurd")
	if err := os.MkdirAll(inst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inst, "instance.cfg"), []byte("[General]\nname=Frangfurd\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := app.GetManifest()
	if err != nil {
		t.Fatal(err)
	}
	idle, err := app.GetGameStates()
	if err != nil || len(idle) != len(manifest.Chapters) {
		t.Fatalf("one state per chapter: %v %+v", err, idle)
	}
	for _, s := range idle {
		if s.Phase != models.GamePhaseIdle || s.ChapterID == "" {
			t.Fatalf("nothing launched yet: %+v", s)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	// No process has pid -1, so the tracker has nothing to find and stays starting.
	err = app.games.Track(ctx, services.TrackRequest{
		ChapterID:   "frangfurd",
		InstanceDir: inst,
		Prism:       services.PrismProcess{PID: -1},
		StartedAt:   time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.LaunchChapter("frangfurd"); err == nil || !strings.Contains(err.Error(), "already starting or running") {
		t.Fatalf("Play is held: %v", err)
	}
	if _, err := app.SaveChapterSettings("frangfurd", models.ChapterSettings{MaxMemoryMB: 4096}); err == nil || !strings.Contains(err.Error(), "running") {
		t.Fatalf("a save is refused: %v", err)
	}
	if info, err := app.GetChapterSettings("frangfurd"); err != nil || !info.Running {
		t.Fatalf("the panel is told: %v %+v", err, info)
	}
	if _, err := app.InstallChapter("frangfurd"); err == nil || !strings.Contains(err.Error(), "starting or running") {
		t.Fatalf("an install is refused: %v", err)
	}
	states, err := app.GetGameStates()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range states {
		want := models.GamePhaseIdle
		if s.ChapterID == "frangfurd" {
			want = models.GamePhaseStarting
		}
		if s.Phase != want {
			t.Fatalf("%s is %s, want %s", s.ChapterID, s.Phase, want)
		}
	}
}

func TestCopyRedactedLogPutsTheMaskedTailOnTheClipboard(t *testing.T) {
	app := newTestApp(t)
	app.home = `C:\Users\sandro`
	app.osUser = "Alessandro"
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", ProfileName: "Sandro_G"}); err != nil {
		t.Fatal(err)
	}
	logText := strings.Join([]string{
		`level=INFO msg=starting dataDir=C:\Users\sandro\AppData\Roaming\KapitalLauncher`,
		`level=INFO msg=launched profile=Sandro_G`,
		`level=WARN msg="ping failed" addr=rails-enjoyed.tun.ply.gg:25565 login=Alessandro`,
	}, "\n") + "\n"
	if err := os.WriteFile(services.LogPath(app.dataDir), []byte(logText), 0o600); err != nil {
		t.Fatal(err)
	}
	var copied string
	app.setClipboard = func(_ context.Context, text string) error { copied = text; return nil }

	lines, err := app.CopyRedactedLog()
	if err != nil || lines != 3 {
		t.Fatalf("%v, %d lines", err, lines)
	}
	for _, leaked := range []string{"sandro", "Sandro_G", "Alessandro", "ply.gg"} {
		if strings.Contains(copied, leaked) {
			t.Errorf("%q reached the clipboard:\n%s", leaked, copied)
		}
	}
	if !strings.Contains(copied, "msg=launched") {
		t.Errorf("the rest of the line must survive:\n%s", copied)
	}
}

func TestCopyRedactedLogReportsWhatWentWrong(t *testing.T) {
	app := newTestApp(t)
	app.setClipboard = func(context.Context, string) error { return errors.New("no clipboard") }
	if _, err := app.CopyRedactedLog(); err == nil {
		t.Fatal("a missing log must be an error, not an empty copy")
	}
	if err := os.WriteFile(services.LogPath(app.dataDir), []byte("a line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CopyRedactedLog(); err == nil || !strings.Contains(err.Error(), "no clipboard") {
		t.Fatalf("a clipboard failure must reach the caller: %v", err)
	}
}
