package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kapital/backend/models"
	"kapital/backend/services"
)

const devPackURL = "http://127.0.0.1:8080/pack.toml"

// commandLine is a PreLaunchCommand line of the launcher's current template
// for a pack URL, as the launcher writes it.
func commandLine(url string) string {
	const head = `"$INST_JAVA" -jar "$INST_MC_DIR/packwiz-installer-bootstrap.jar" ` +
		`--bootstrap-no-update --bootstrap-main-jar "$INST_MC_DIR/packwiz-installer.jar" -g `
	return `PreLaunchCommand="` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(head+url) + `"`
}

// packApp is an app with Prism found and Frangfurd installed with the given
// PreLaunchCommand line; it returns the app and the instance.cfg path.
func packApp(t *testing.T, line string, overrides map[string]string) (*App, string) {
	t.Helper()
	app := newTestApp(t)
	root := t.TempDir()
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", PrismRoot: root, PackOverrides: overrides}); err != nil {
		t.Fatal(err)
	}
	app.engine = models.EngineInfo{Found: true, Source: "settings"}
	inst := filepath.Join(root, "instances", "kapital-frangfurd")
	if err := os.MkdirAll(inst, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(inst, "instance.cfg")
	if err := os.WriteFile(cfg, []byte("[General]\r\nname=Frangfurd\r\n"+line+"\r\niconKey=default\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return app, cfg
}

func publishedPackURL(t *testing.T, app *App) string {
	t.Helper()
	m, err := app.GetManifest()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range m.Chapters {
		if c.ID == "frangfurd" && c.Pack.Packwiz != nil {
			return *c.Pack.Packwiz
		}
	}
	t.Fatal("frangfurd hosts no pack in the bundled manifest")
	return ""
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSetPackSourceSwitchesBothWaysAndReportsTheNewURL(t *testing.T) {
	app, cfg := packApp(t, commandLine("placeholder"), map[string]string{"frangfurd": devPackURL})
	published := publishedPackURL(t, app)
	if err := os.WriteFile(cfg, []byte("[General]\r\nname=Frangfurd\r\n"+commandLine(published)+"\r\niconKey=default\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct{ source, url string }{
		{"dev", devPackURL},
		{"dev", devPackURL},
		{"published", published},
	} {
		report, err := app.SetPackSource("frangfurd", step.source)
		if err != nil {
			t.Fatalf("%s: %v", step.source, err)
		}
		if report.PackURL["frangfurd"] != step.url {
			t.Fatalf("%s: the report says %q", step.source, report.PackURL["frangfurd"])
		}
		want := "[General]\r\nname=Frangfurd\r\n" + commandLine(step.url) + "\r\niconKey=default\r\n"
		if got := readFile(t, cfg); got != want {
			t.Fatalf("%s: instance.cfg:\n%q\nwant:\n%q", step.source, got, want)
		}
	}
}

func TestSetPackSourceRefusesBeforeWritingAnything(t *testing.T) {
	app, cfg := packApp(t, commandLine("https://example.com/pack.toml"), nil)
	before := readFile(t, cfg)
	check := func(name string, want string, err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v, want %q", name, err, want)
		}
		if readFile(t, cfg) != before {
			t.Fatalf("%s: the file changed:\n%q", name, readFile(t, cfg))
		}
	}
	_, err := app.SetPackSource("atlantis", "dev")
	check("an unknown chapter", "no chapter", err)
	_, err = app.SetPackSource("frangfurd", "staging")
	check("an unknown source", "unknown pack source", err)
	_, err = app.SetPackSource("frangfurd", "")
	check("an empty source", "unknown pack source", err)
	_, err = app.SetPackSource("frangfurd", "dev")
	check("no override in settings", "no local pack", err)
	_, err = app.SetPackSource("lichdenstein", "published")
	check("a chapter not installed", "not installed", err)

	// Lichdenstein hosts no pack yet; installed, it has no published one.
	inst := filepath.Join(filepath.Dir(filepath.Dir(cfg)), "kapital-lichdenstein")
	if err := os.MkdirAll(inst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inst, "instance.cfg"), []byte("[General]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = app.SetPackSource("lichdenstein", "published")
	check("no published pack", "no published pack", err)

	// A command the launcher did not write is refused, not rewritten.
	app, cfg = packApp(t, `PreLaunchCommand="echo hi"`, map[string]string{"frangfurd": devPackURL})
	before = readFile(t, cfg)
	_, err = app.SetPackSource("frangfurd", "dev")
	check("a hand-edited command", "did not write", err)
	if err != nil && strings.Contains(err.Error(), "echo") {
		t.Fatalf("the error carries the command: %v", err)
	}

	app.engine = models.EngineInfo{}
	_, err = app.SetPackSource("frangfurd", "dev")
	check("no Prism", services.ErrPrismNotFound.Error(), err)
}

func TestSetPackSourceHoldsTheOverrideToTheLoopback(t *testing.T) {
	// Settings load drops a bad override, so the check is for a file written
	// behind the app's back; either way nothing reaches the instance.
	app, cfg := packApp(t, commandLine("https://example.com/pack.toml"), map[string]string{"frangfurd": devPackURL})
	before := readFile(t, cfg)
	path := filepath.Join(app.dataDir, services.SettingsFileName)
	if err := os.WriteFile(path, []byte(`{"theme":"dark","packOverrides":{"frangfurd":"http://evil.example/pack.toml"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SetPackSource("frangfurd", "dev"); err == nil {
		t.Fatal("a non-loopback override was taken")
	}
	if readFile(t, cfg) != before {
		t.Fatalf("the file changed:\n%q", readFile(t, cfg))
	}
}

func TestSetPackSourceIsRefusedWhileTheGameIsActive(t *testing.T) {
	app, cfg := packApp(t, commandLine("https://example.com/pack.toml"), map[string]string{"frangfurd": devPackURL})
	before := readFile(t, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	// No process has pid -1, so the tracker has nothing to find and stays starting.
	err := app.games.Track(ctx, services.TrackRequest{
		ChapterID:   "frangfurd",
		InstanceDir: filepath.Dir(cfg),
		Prism:       services.PrismProcess{PID: -1},
		StartedAt:   time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.SetPackSource("frangfurd", "dev"); err == nil || err.Error() != "Frangfurd is starting or running; close the game first" {
		t.Fatalf("got %v", err)
	}
	if readFile(t, cfg) != before {
		t.Fatalf("the file changed:\n%q", readFile(t, cfg))
	}
}

func TestSetPackSourceIsRefusedWhileAGameStartedFromPrismLooksToBeRunning(t *testing.T) {
	app, cfg := packApp(t, commandLine("https://example.com/pack.toml"), map[string]string{"frangfurd": devPackURL})
	before := readFile(t, cfg)
	logs := filepath.Join(filepath.Dir(cfg), "minecraft", "logs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logs, "latest.log"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SetPackSource("frangfurd", "dev"); err == nil || !strings.Contains(err.Error(), "game log changed less than a minute ago") {
		t.Fatalf("got %v", err)
	}
	if readFile(t, cfg) != before {
		t.Fatalf("the file changed:\n%q", readFile(t, cfg))
	}
}

// The loading card names the version the pack source served at the last
// read, not the one built into the manifest, and an unread source leaves the
// manifest's.
func TestTheCardsPackVersionIsTheSourcesLastRead(t *testing.T) {
	app := newTestApp(t)
	chapter, ok := app.chapter("frangfurd")
	if !ok {
		t.Fatal("no frangfurd")
	}
	manifest := ""
	if chapter.Pack.Version != nil {
		manifest = *chapter.Pack.Version
	}

	app.notePackVersion(models.PackState{ChapterID: "frangfurd", Installed: true, Checked: false, Version: "9.9.9"})
	if got := app.withPackVersion(chapter).Pack.Version; manifest != "" && (got == nil || *got != manifest) {
		t.Fatalf("an unread source replaced the manifest's version: %v", got)
	}

	app.notePackVersion(models.PackState{ChapterID: "frangfurd", Installed: true, Checked: true, Version: "1.0.7"})
	if got := app.withPackVersion(chapter).Pack.Version; got == nil || *got != "1.0.7" {
		t.Fatalf("got %v", got)
	}
	if again, _ := app.chapter("frangfurd"); again.Pack.Version != chapter.Pack.Version {
		t.Fatal("the manifest itself was changed")
	}
}
