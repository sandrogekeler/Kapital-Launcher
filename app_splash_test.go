package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"kapital/backend/design"
	"kapital/backend/models"
	"kapital/backend/services"
	"kapital/backend/splashhost"
)

// eventLog is the game:state events the app emitted. The card's own messages
// are handled on a goroutine, so it is safe to read while they arrive.
type eventLog struct {
	mu   sync.Mutex
	list []models.GameState
}

func (e *eventLog) add(s models.GameState) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.list = append(e.list, s)
}

func (e *eventLog) all() []models.GameState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]models.GameState(nil), e.list...)
}

func (e *eventLog) last() models.GameState {
	all := e.all()
	if len(all) == 0 {
		return models.GameState{}
	}
	return all[len(all)-1]
}

// cardApp is an app whose card opens in a fake window over a fake build.
func cardApp(t *testing.T) (*App, *splashhost.Fake, *eventLog) {
	t.Helper()
	app := newTestApp(t)
	host := &splashhost.Fake{}
	dist := fstest.MapFS{"splash.html": {Data: []byte("<html>")}}
	// No clipboard to reach from a test.
	app.setClipboard = func(context.Context, string) error { return nil }
	app.splash = app.newSplashCard(dist, func() splashhost.Host { return host })
	events := &eventLog{}
	app.emit = events.add
	return app, host, events
}

func beginCard(t *testing.T, app *App, chapterID string) {
	t.Helper()
	chapter, ok := app.chapter(chapterID)
	if !ok {
		t.Fatalf("no chapter %s", chapterID)
	}
	if !app.splash.Begin(chapter, "dark") {
		t.Fatal("the card did not open")
	}
}

func lastState(t *testing.T, host *splashhost.Fake) splashhost.State {
	t.Helper()
	var s splashhost.State
	updates := host.Updates()
	if len(updates) == 0 {
		return s
	}
	if err := json.Unmarshal(updates[len(updates)-1], &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

// Without a window the launcher's calls do nothing and nothing panics: the
// Wails runtime would stop the process on a missing context.
func TestTheLauncherWindowsCallsAreNoOpsBeforeTheWindowExists(t *testing.T) {
	w := launcherWindow{newTestApp(t)}
	w.Minimise()
	w.Unminimise()
	if x, y, width, height := w.Frame(); x != 0 || y != 0 || width != 0 || height != 0 {
		t.Fatalf("%d,%d %dx%d", x, y, width, height)
	}
}

func TestNoCardOpensWithoutAWindowToCentreOn(t *testing.T) {
	app, host, _ := cardApp(t)
	chapter, _ := app.chapter("frangfurd")
	on := true
	if app.beginSplash(chapter, models.AppSettings{LoadingSplash: &on}) {
		t.Fatal("before startup there is no window")
	}
	if len(host.Calls()) != 0 {
		t.Fatalf("%v", host.Calls())
	}
}

// The card is made from the app's own pieces: the embedded build, its data
// folder, the dark ground, and the two actions.
func TestTheCardIsWiredToTheEmbeddedBuildAndTheAppsActions(t *testing.T) {
	app, host, _ := cardApp(t)
	beginCard(t, app, "luxemburg")

	page := host.Page()
	if page.Entry != "splash.html" || page.Background != design.WindowBackground ||
		page.DataDir != filepath.Join(app.dataDir, "splash-webview") {
		t.Fatalf("%+v", page)
	}
	if _, mime, ok := page.Assets("splash.html"); !ok || mime != "text/html; charset=utf-8" {
		t.Fatal("the entry is served from the build")
	}
	if _, _, ok := page.Assets("../app.go"); ok {
		t.Fatal("anything outside the build is refused")
	}

	// The page's actions run the app's own: no Prism here, so the folder
	// cannot be found, and that is what the card then says.
	host.Message(`{"action":"openFolder"}`)
	waitFor(t, "the folder's refusal on the card", func() bool { return lastState(t, host).Error != "" })
	host.Message(`{"action":"copyLog"}`)
	waitFor(t, "the log's outcome on the card", func() bool { return lastState(t, host).CopyLog != nil })
}

func TestLeaveSplashWithNoCardUpIsANoOp(t *testing.T) {
	app, host, events := cardApp(t)
	if err := app.LeaveSplash(); err != nil {
		t.Fatal(err)
	}
	if len(host.Calls()) != 0 || len(events.all()) != 0 {
		t.Fatalf("nothing to close or announce: %v %v", host.Calls(), events.all())
	}
}

// GetGameStates carries the splash flag the events carry: true for the
// chapter whose card is up, until the player leaves it, and LeaveSplash says
// the card is gone.
func TestGetGameStatesCarriesTheSplashFlagAndLeaveSplashClearsIt(t *testing.T) {
	app, host, events := cardApp(t)
	beginCard(t, app, "frangfurd")

	flags := func() map[string]bool {
		states, err := app.GetGameStates()
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, s := range states {
			out[s.ChapterID] = s.Splash
		}
		return out
	}
	for id, splash := range flags() {
		if splash != (id == "frangfurd") {
			t.Fatalf("only Frangfurd's card is up: %v", flags())
		}
	}

	if err := app.LeaveSplash(); err != nil {
		t.Fatal(err)
	}
	for id, splash := range flags() {
		if splash {
			t.Fatalf("%s still shows the card", id)
		}
	}
	if host.Closes() != 1 {
		t.Fatalf("the card closed: %v", host.Calls())
	}
	if all := events.all(); len(all) != 1 || all[0].Splash || all[0].ChapterID != "frangfurd" {
		t.Fatalf("an updated state with no splash follows at once: %+v", all)
	}
	// A second leave has nothing to announce.
	if err := app.LeaveSplash(); err != nil || len(events.all()) != 1 {
		t.Fatalf("%v %d", err, len(events.all()))
	}
}

// The tracker's events are decorated with the splash flag, the card follows
// them, and one that crashes before the handover keeps the card for the error.
func TestGameEventsAreDecoratedWithTheSplashFlagAndFollowedByTheCard(t *testing.T) {
	app, host, events := cardApp(t)
	beginCard(t, app, "frangfurd")
	updates := len(host.Updates())

	app.onGameState(models.GameState{ChapterID: "frangfurd", Phase: models.GamePhaseMods})
	app.onGameState(models.GameState{ChapterID: "frangfurd", Phase: models.GamePhaseCrashed})
	if all := events.all(); len(all) != 2 || !all[0].Splash || !all[1].Splash {
		t.Fatalf("a crash before the handover keeps the card: %+v", all)
	}
	if len(host.Updates()) != updates+2 || host.Closes() != 0 {
		t.Fatalf("the card follows: %v", host.Calls())
	}

	// Another chapter's events are not the card's.
	app.onGameState(models.GameState{ChapterID: "luxemburg", Phase: models.GamePhaseMods})
	if events.last().Splash {
		t.Fatalf("%+v", events.last())
	}

	// The player leaves from the card, as a message from the page: the view is
	// told at once.
	host.Message(`{"action":"leave"}`)
	waitFor(t, "the card to close", func() bool { return host.Closes() == 1 })
	waitFor(t, "the view to be told", func() bool {
		last := events.last()
		return !last.Splash && last.ChapterID == "frangfurd"
	})
}

// A run that ends well after the handover says there is no card, and the card
// is not touched again.
func TestTheEndOfAHandedOverRunSaysThereIsNoCard(t *testing.T) {
	app, host, events := cardApp(t)
	beginCard(t, app, "frangfurd")
	if runtime.GOOS == "windows" {
		app.splash.Handover("frangfurd")
	} else {
		app.onGameState(models.GameState{ChapterID: "frangfurd", Phase: models.GamePhaseWindow})
	}
	app.onGameState(models.GameState{ChapterID: "frangfurd", Phase: models.GamePhaseClosed})
	if last := events.last(); last.Splash || last.Phase != models.GamePhaseClosed {
		t.Fatalf("%+v", last)
	}
	if host.Closes() != 1 {
		t.Fatalf("%v", host.Calls())
	}
}

func TestGetSettingsReportsWhatTheSplashComesToOnThisOS(t *testing.T) {
	app := newTestApp(t)
	got, err := app.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	available := services.LoadingSplashAvailable(runtime.GOOS)
	on := services.LoadingSplashOn(runtime.GOOS, models.AppSettings{})
	if got.LoadingSplashAvailable != available || got.LoadingSplashOn != on {
		t.Fatalf("on %v: %+v", runtime.GOOS, got)
	}
	off := false
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", LoadingSplash: &off}); err != nil {
		t.Fatal(err)
	}
	got, err = app.GetSettings()
	if err != nil || got.LoadingSplashOn || got.LoadingSplashAvailable != available {
		t.Fatalf("explicitly off: %v %+v", err, got)
	}
}
