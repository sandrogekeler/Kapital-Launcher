package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	legacyCommand   = `PreLaunchCommand="\"$INST_JAVA\" -jar \"$INST_MC_DIR/packwiz-installer-bootstrap.jar\" --bootstrap-no-update --bootstrap-main-jar \"$INST_MC_DIR/packwiz-installer.jar\" https://kapitel-kapital.pages.dev/frangfurd/pack.toml"`
	headlessCommand = `PreLaunchCommand="\"$INST_JAVA\" -jar \"$INST_MC_DIR/packwiz-installer-bootstrap.jar\" --bootstrap-no-update --bootstrap-main-jar \"$INST_MC_DIR/packwiz-installer.jar\" -g https://kapitel-kapital.pages.dev/frangfurd/pack.toml"`
)

func TestUpdatePreLaunchBringsAnEarlierInstanceUpAndNeverBlocksTheLaunch(t *testing.T) {
	a := &App{}
	cfg := func(dir string) string { return filepath.Join(dir, "instance.cfg") }
	write := func(dir, command string) {
		t.Helper()
		if err := os.WriteFile(cfg(dir), []byte("[General]\r\nname=x\r\n"+command+"\r\niconKey=default\r\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(dir string) string {
		t.Helper()
		b, err := os.ReadFile(cfg(dir))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	dir := t.TempDir()
	write(dir, legacyCommand)
	a.updatePreLaunch("frangfurd", dir)
	if want := "[General]\r\nname=x\r\n" + headlessCommand + "\r\niconKey=default\r\n"; read(dir) != want {
		t.Fatalf("instance.cfg:\n%q\nwant:\n%q", read(dir), want)
	}

	// Hand-edited: left alone.
	edited := t.TempDir()
	write(edited, strings.Replace(legacyCommand, "--bootstrap-no-update", "--bootstrap-no-update -Xmx1G", 1))
	before := read(edited)
	a.updatePreLaunch("frangfurd", edited)
	if read(edited) != before {
		t.Fatalf("a hand-edited command changed:\n%q", read(edited))
	}

	// A game that looks to be running: Prism may be writing the file.
	running := t.TempDir()
	write(running, legacyCommand)
	logs := filepath.Join(running, "minecraft", "logs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logs, "latest.log"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	before = read(running)
	a.updatePreLaunch("frangfurd", running)
	if read(running) != before {
		t.Fatalf("a running instance was written:\n%q", read(running))
	}

	// No instance, or no folder known: nothing to do.
	a.updatePreLaunch("frangfurd", filepath.Join(t.TempDir(), "kapital-frangfurd"))
	a.updatePreLaunch("frangfurd", "")
}
