package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kapital/backend/models"
	"kapital/backend/services"
)

// installedFrangfurd makes the chapter's instance in a fresh Prism root and
// returns its game folder.
func installedFrangfurd(t *testing.T, app *App) string {
	t.Helper()
	root := t.TempDir()
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", PrismRoot: root}); err != nil {
		t.Fatal(err)
	}
	inst := filepath.Join(root, "instances", "kapital-frangfurd")
	game := filepath.Join(inst, "minecraft")
	for _, dir := range []string{"logs", "crash-reports"} {
		if err := os.MkdirAll(filepath.Join(game, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(inst, "instance.cfg"), []byte("[General]\nname=Frangfurd\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return game
}

func TestRunLogsRefuseAnUnknownChapter(t *testing.T) {
	app := newTestApp(t)
	if _, err := app.GetRunLogs("atlantis"); err == nil || !strings.Contains(err.Error(), "no chapter") {
		t.Fatalf("got %v", err)
	}
	if _, err := app.ReadRunLog("atlantis", models.RunLogKindLog, "latest.log", 0); err == nil || !strings.Contains(err.Error(), "no chapter") {
		t.Fatalf("got %v", err)
	}
}

// A chapter that is not installed has no logs, which is an answer and not an
// error; reading one of its files is refused.
func TestRunLogsOfAChapterThatIsNotInstalledAreEmpty(t *testing.T) {
	app := newTestApp(t)
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", PrismRoot: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	logs, err := app.GetRunLogs("frangfurd")
	if err != nil || logs == nil || len(logs) != 0 {
		t.Fatalf("got %v, %v; want an empty list and no error", logs, err)
	}
	if _, err := app.ReadRunLog("frangfurd", models.RunLogKindLog, "latest.log", 0); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("got %v", err)
	}
}

func TestRunLogsListAndReadAnInstalledChaptersFilesMasked(t *testing.T) {
	app := newTestApp(t)
	app.home, app.osUser = `C:\Users\sandro`, "sandro"
	game := installedFrangfurd(t, app)
	if err := os.WriteFile(filepath.Join(game, "logs", "latest.log"), []byte("Game dir C:\\Users\\sandro\\x\nwho: sandro\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(game, "crash-reports", "crash-a.txt"), []byte("boom\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	logs, err := app.GetRunLogs("frangfurd")
	if err != nil || len(logs) != 2 {
		t.Fatalf("got %v, %v", logs, err)
	}
	text, err := app.ReadRunLog("frangfurd", models.RunLogKindLog, "latest.log", 0)
	if err != nil {
		t.Fatal(err)
	}
	if text.Text != "Game dir [home]\\x\nwho: [user]\n" || text.Lines != 2 {
		t.Fatalf("masked before it left Go: %q", text.Text)
	}
	crash, err := app.ReadRunLog("frangfurd", models.RunLogKindCrash, "crash-a.txt", 0)
	if err != nil || crash.Text != "boom\n" {
		t.Fatalf("got %+v, %v", crash, err)
	}
}

// The bridge names a kind and a base name, never a path.
func TestReadRunLogRefusesWhatIsNotAListedName(t *testing.T) {
	app := newTestApp(t)
	game := installedFrangfurd(t, app)
	if err := os.WriteFile(filepath.Join(filepath.Dir(game), "instance.cfg"), []byte("[General]\nname=X\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../../instance.cfg", `..\..\instance.cfg`, "../logs/latest.log", "instance.cfg", "/etc/passwd", "latest.log/../../x"} {
		_, err := app.ReadRunLog("frangfurd", models.RunLogKindLog, name, 0)
		if !errors.Is(err, services.ErrRunLogName) {
			t.Errorf("%q: got %v", name, err)
		}
	}
	if _, err := app.ReadRunLog("frangfurd", "instance", "instance.cfg", 0); !errors.Is(err, services.ErrRunLogName) {
		t.Errorf("a kind that is not log or crash: %v", err)
	}
}

func TestALogsErrorShownToThePageIsMasked(t *testing.T) {
	app := newTestApp(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home folder to mask")
	}
	chapter, _ := app.chapter("frangfurd")
	cause := fmt.Errorf("open game folder: open %s: access denied", filepath.Join(home, "AppData", "x"))
	got := app.maskedError(chapter, cause)
	if strings.Contains(got.Error(), home) {
		t.Fatalf("the home path reached the page: %q", got)
	}
	if !errors.Is(got, cause) {
		t.Fatal("the masked error must still wrap its cause")
	}
}
