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

const (
	trackerRefusal = "Frangfurd is starting or running; close the game first"
	guessRefusal   = "Frangfurd's game log changed less than a minute ago: if the game is closed, try again in a moment"
)

// touchGameLog writes the instance's latest.log and dates it that long ago.
func touchGameLog(t *testing.T, cfg string, age time.Duration) {
	t.Helper()
	logs := filepath.Join(filepath.Dir(cfg), "minecraft", "logs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(logs, "latest.log")
	if err := os.WriteFile(log, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(log, when, when); err != nil {
		t.Fatal(err)
	}
}

// trackFrangfurd makes the tracker hold Frangfurd as starting.
func trackFrangfurd(t *testing.T, app *App, cfg string) {
	t.Helper()
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
}

var anySave = models.ChapterSettings{MaxMemoryMB: 4096, JVM: "zgc"}

// The tracker is the exact answer and says so in its own words (#126).
func TestASettingsSaveRefusedByTheTrackerSaysSo(t *testing.T) {
	app, cfg := packApp(t, commandLine("https://example.com/pack.toml"), nil)
	trackFrangfurd(t, app, cfg)
	before := readFile(t, cfg)
	if _, err := app.SaveChapterSettings("frangfurd", anySave); err == nil || err.Error() != trackerRefusal {
		t.Fatalf("got %v", err)
	}
	if readFile(t, cfg) != before {
		t.Fatal("the file changed")
	}
}

// The guess alone, a log touched just now with the tracker idle, refuses with
// the second message, and a log older than the window does not (#126).
func TestASettingsSaveRefusedByTheGuessAloneSaysSo(t *testing.T) {
	app, cfg := packApp(t, commandLine("https://example.com/pack.toml"), nil)
	before := readFile(t, cfg)
	touchGameLog(t, cfg, 0)
	if _, err := app.SaveChapterSettings("frangfurd", anySave); err == nil || err.Error() != guessRefusal {
		t.Fatalf("got %v", err)
	}
	if readFile(t, cfg) != before {
		t.Fatal("the file changed")
	}
	info, err := app.GetChapterSettings("frangfurd")
	if err != nil || !info.Running {
		t.Fatalf("the panel is told as a hint: %v %+v", err, info)
	}

	touchGameLog(t, cfg, 2*time.Minute)
	saved, err := app.SaveChapterSettings("frangfurd", anySave)
	if err != nil || saved.Settings.MaxMemoryMB != 4096 || saved.Running {
		t.Fatalf("a log older than the window is not running: %v %+v", err, saved)
	}
}

func TestSetPackSourceRefusedByTheGuessAloneSaysSoAndAnOldLogDoesNot(t *testing.T) {
	app, cfg := packApp(t, commandLine("https://example.com/pack.toml"), map[string]string{"frangfurd": devPackURL})
	touchGameLog(t, cfg, 0)
	if _, err := app.SetPackSource("frangfurd", "dev"); err == nil || err.Error() != guessRefusal {
		t.Fatalf("got %v", err)
	}
	touchGameLog(t, cfg, 2*time.Minute)
	if _, err := app.SetPackSource("frangfurd", "dev"); err != nil {
		t.Fatalf("a log older than the window is not running: %v", err)
	}
	if !strings.Contains(readFile(t, cfg), devPackURL) {
		t.Fatal("the switch was not written")
	}
}

// With both answers true, the tracker is the one named.
func TestTheTrackerIsNamedWhenBothAnswersSayRunning(t *testing.T) {
	app, cfg := packApp(t, commandLine("https://example.com/pack.toml"), nil)
	trackFrangfurd(t, app, cfg)
	touchGameLog(t, cfg, 0)
	if _, err := app.SaveChapterSettings("frangfurd", anySave); err == nil || err.Error() != trackerRefusal {
		t.Fatalf("got %v", err)
	}
}

// A game the tracker saw close frees the instance at once: its log, written
// as it closed, is not the guess's. A later write still is, a start from
// Prism itself.
func TestALogTheTrackerSawEndWithDoesNotRefuse(t *testing.T) {
	app, cfg := packApp(t, commandLine("https://example.com/pack.toml"), map[string]string{"frangfurd": devPackURL})
	touchGameLog(t, cfg, 2*time.Second)
	app.onGameState(models.GameState{ChapterID: "frangfurd", Phase: models.GamePhaseClosed})
	if _, err := app.SetPackSource("frangfurd", "dev"); err != nil {
		t.Fatalf("the closed run's log refused the switch: %v", err)
	}
	info, err := app.GetChapterSettings("frangfurd")
	if err != nil || info.Running {
		t.Fatalf("the panel still hints at a game: %v %+v", err, info)
	}

	touchGameLog(t, cfg, -10*time.Second)
	if _, err := app.SetPackSource("frangfurd", "published"); err == nil || err.Error() != guessRefusal {
		t.Fatalf("a write after the end is a game again: %v", err)
	}
}
