package main

import (
	"os"
	"path/filepath"
	"testing"

	"kapital/backend/models"
)

// Play time (issue 192) is read for the installed chapters only, from the two
// keys Prism keeps in each instance.cfg.
func TestGetPlayTimeReadsInstalledChaptersOnly(t *testing.T) {
	app := newTestApp(t)
	root := t.TempDir()
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", PrismRoot: root}); err != nil {
		t.Fatal(err)
	}
	if got, err := app.GetPlayTime(); err != nil || len(got) != 0 {
		t.Fatalf("nothing installed: %v %+v", err, got)
	}
	inst := filepath.Join(root, "instances", "kapital-frangfurd")
	if err := os.MkdirAll(inst, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "[General]\nname=Frangfurd\ntotalTimePlayed=229200\nlastLaunchTime=1759660800000\n"
	if err := os.WriteFile(filepath.Join(inst, "instance.cfg"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := app.GetPlayTime()
	if err != nil {
		t.Fatal(err)
	}
	want := models.PlayTime{ChapterID: "frangfurd", TotalSeconds: 229200, LastLaunchMs: 1759660800000}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %+v, want [%+v]", got, want)
	}
}
