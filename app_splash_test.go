package main

import (
	"runtime"
	"testing"

	"kapital/backend/design"
	"kapital/backend/models"
	"kapital/backend/services"
)

// recordingWindow is a window that does as it is told at once and remembers
// the calls that matter.
type recordingWindow struct {
	w, h  int
	calls []string
}

func (r *recordingWindow) GetSize() (int, int)     { return r.w, r.h }
func (r *recordingWindow) GetPosition() (int, int) { return 10, 20 }
func (r *recordingWindow) IsMaximised() bool       { return false }
func (r *recordingWindow) Maximise()               { r.calls = append(r.calls, "Maximise") }
func (r *recordingWindow) Unmaximise()             { r.calls = append(r.calls, "Unmaximise") }
func (r *recordingWindow) SetMinSize(int, int)     { r.calls = append(r.calls, "SetMinSize") }
func (r *recordingWindow) SetSize(w, h int) {
	r.calls = append(r.calls, "SetSize")
	r.w, r.h = w, h
}
func (r *recordingWindow) SetPosition(int, int) { r.calls = append(r.calls, "SetPosition") }
func (r *recordingWindow) Center()              { r.calls = append(r.calls, "Center") }
func (r *recordingWindow) Minimise()            { r.calls = append(r.calls, "Minimise") }
func (r *recordingWindow) Unminimise()          { r.calls = append(r.calls, "Unminimise") }

// Without a window the splash's calls do nothing and nothing panics: the
// Wails runtime would stop the process on a missing context.
func TestTheSplashesWindowCallsAreNoOpsBeforeTheWindowExists(t *testing.T) {
	app := newTestApp(t)
	w := appWindow{app}
	w.Minimise()
	w.Unminimise()
	w.Maximise()
	w.Unmaximise()
	w.Center()
	w.SetMinSize(0, 0)
	w.SetSize(1, 1)
	w.SetPosition(1, 1)
	if width, height := w.GetSize(); width != 0 || height != 0 {
		t.Fatalf("%dx%d", width, height)
	}
	if x, y := w.GetPosition(); x != 0 || y != 0 || w.IsMaximised() {
		t.Fatalf("%d,%d", x, y)
	}
}

func TestLeaveSplashWithNoCardUpIsANoOp(t *testing.T) {
	app := newTestApp(t)
	win := &recordingWindow{w: 1000, h: 600}
	app.splash = services.NewSplashWindow(win)
	if err := app.LeaveSplash(); err != nil {
		t.Fatal(err)
	}
	if len(win.calls) != 0 {
		t.Fatalf("nothing to restore: %v", win.calls)
	}
}

// GetGameStates carries the splash flag the events carry: true for the
// chapter whose card is up, until the player leaves it.
func TestGetGameStatesCarriesTheSplashFlagAndLeaveSplashClearsIt(t *testing.T) {
	app := newTestApp(t)
	win := &recordingWindow{w: 1000, h: 600}
	app.splash = services.NewSplashWindow(win)
	app.splash.Enter("frangfurd")
	if win.w != design.SplashWidth || win.h != design.SplashHeight {
		t.Fatalf("the window became %dx%d", win.w, win.h)
	}

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
	got := flags()
	for id, splash := range got {
		if splash != (id == "frangfurd") {
			t.Fatalf("only Frangfurd's card is up: %v", got)
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
	if win.w != 1000 || win.h != 600 {
		t.Fatalf("the window is back at %dx%d", win.w, win.h)
	}
}

// The tracker's events are decorated with the splash flag, a run that ends
// well gives the window back before its event says the card is gone, and one
// that crashes before the handover keeps the card for the error state.
func TestGameEventsAreDecoratedWithTheSplashFlag(t *testing.T) {
	app := newTestApp(t)
	win := &recordingWindow{w: 1000, h: 600}
	app.splash = services.NewSplashWindow(win)
	var events []models.GameState
	app.emit = func(s models.GameState) { events = append(events, s) }
	app.splash.Enter("frangfurd")
	win.calls = nil

	app.onGameState(models.GameState{ChapterID: "frangfurd", Phase: models.GamePhaseMods})
	app.onGameState(models.GameState{ChapterID: "frangfurd", Phase: models.GamePhaseCrashed})
	if len(events) != 2 || !events[0].Splash || !events[1].Splash {
		t.Fatalf("a crash before the handover keeps the card: %+v", events)
	}
	if len(win.calls) != 0 {
		t.Fatalf("and the window: %v", win.calls)
	}

	// The player leaves; LeaveSplash announces it for the chapter.
	if err := app.LeaveSplash(); err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[2].Splash || events[2].ChapterID != "frangfurd" {
		t.Fatalf("an updated state with no splash follows at once: %+v", events)
	}
	if win.w != 1000 || win.h != 600 {
		t.Fatalf("the window is back at %dx%d", win.w, win.h)
	}

	// A second leave has nothing to announce.
	if err := app.LeaveSplash(); err != nil || len(events) != 3 {
		t.Fatalf("%v %d", err, len(events))
	}

	// A run that ends well after the handover gives the window back first.
	app.splash.Enter("frangfurd")
	app.splash.Handover("frangfurd")
	win.calls = nil
	app.onGameState(models.GameState{ChapterID: "frangfurd", Phase: models.GamePhaseClosed})
	if last := events[len(events)-1]; last.Splash || last.Phase != models.GamePhaseClosed {
		t.Fatalf("%+v", last)
	}
	if len(win.calls) == 0 || win.calls[0] != "Unminimise" {
		t.Fatalf("the window was given back: %v", win.calls)
	}
}

func TestGetSettingsReportsWhatTheSplashComesToOnThisOS(t *testing.T) {
	app := newTestApp(t)
	got, err := app.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	want := services.LoadingSplashAvailable(runtime.GOOS)
	if got.LoadingSplashAvailable != want || got.LoadingSplashOn != want {
		t.Fatalf("default on where available (%v): %+v", want, got)
	}
	off := false
	if err := app.SaveSettings(models.AppSettings{Theme: "dark", LoadingSplash: &off}); err != nil {
		t.Fatal(err)
	}
	got, err = app.GetSettings()
	if err != nil || got.LoadingSplashOn || got.LoadingSplashAvailable != want {
		t.Fatalf("explicitly off: %v %+v", err, got)
	}
}
