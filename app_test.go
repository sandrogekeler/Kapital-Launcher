package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kapital/backend/models"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	app, err := NewApp(t.TempDir(), bundledManifest)
	if err != nil {
		t.Fatal(err)
	}
	return app
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
	src, err := os.ReadFile("app.go")
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
			t.Errorf("%s does not return an error as its last value", name)
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
	// The bundled manifest hosts no pack yet (#25): Install has nothing to write.
	app.engine = models.EngineInfo{Found: true, Source: "settings"}
	if _, err := app.InstallChapter("frangfurd"); err == nil || !strings.Contains(err.Error(), "no hosted pack") {
		t.Errorf("expected the missing pack to be refused, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "instances", "kapital-frangfurd")); !os.IsNotExist(err) {
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
