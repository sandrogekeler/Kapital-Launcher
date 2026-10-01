package services

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"kapital/backend/design"
	"kapital/backend/models"
)

// fakeWindow is the launcher's window: it keeps a size, a place and a
// maximised flag, answers a size only after a resize has "landed" some polls
// later, and records every call that changes it, in order.
type fakeWindow struct {
	calls []string

	w, h, x, y int
	maximised  bool
	// maxW, maxH is what a maximised window reports, and the restored size
	// is w, h.
	maxW, maxH int
	// lag is how many GetSize calls still answer the old size after SetSize.
	lag int
	// polls is how many GetSize calls there have been, and centredAt how many
	// there had been when Center was called.
	polls, centredAt int
	pendingW         int
	pendingH         int
	pending          bool
}

func newFakeWindow() *fakeWindow {
	return &fakeWindow{w: 1100, h: 700, x: 300, y: 120, maxW: 2560, maxH: 1400}
}

func (f *fakeWindow) record(call string) { f.calls = append(f.calls, call) }

func (f *fakeWindow) GetSize() (int, int) {
	f.polls++
	if f.pending {
		if f.lag > 0 {
			f.lag--
		} else {
			f.w, f.h, f.pending = f.pendingW, f.pendingH, false
		}
	}
	if f.maximised {
		return f.maxW, f.maxH
	}
	return f.w, f.h
}

func (f *fakeWindow) GetPosition() (int, int) { return f.x, f.y }
func (f *fakeWindow) IsMaximised() bool       { return f.maximised }

func (f *fakeWindow) Maximise() {
	f.record("Maximise")
	f.maximised = true
}

func (f *fakeWindow) Unmaximise() {
	f.record("Unmaximise")
	f.maximised = false
}

func (f *fakeWindow) SetMinSize(w, h int) { f.record("SetMinSize " + dims(w, h)) }

func (f *fakeWindow) SetSize(w, h int) {
	f.record("SetSize " + dims(w, h))
	f.pendingW, f.pendingH, f.pending = w, h, true
}

func (f *fakeWindow) SetPosition(x, y int) {
	f.record("SetPosition " + dims(x, y))
	f.x, f.y = x, y
}

func (f *fakeWindow) Center() {
	f.record("Center")
	f.centredAt = f.polls
}

func (f *fakeWindow) Minimise()   { f.record("Minimise") }
func (f *fakeWindow) Unminimise() { f.record("Unminimise") }

func dims(a, b int) string { return fmt.Sprintf("%dx%d", a, b) }

// splashRig is a SplashWindow over a fake window that never really sleeps.
type splashRig struct {
	win    *fakeWindow
	splash *SplashWindow
	sleeps int
}

func newSplashRig() *splashRig {
	r := &splashRig{win: newFakeWindow()}
	r.splash = NewSplashWindow(r.win)
	r.splash.sleep = func(time.Duration) { r.sleeps++ }
	return r
}

var (
	splashSize  = "SetSize " + dims(design.SplashWidth, design.SplashHeight)
	minSize     = "SetMinSize " + dims(design.WindowMinWidth, design.WindowMinHeight)
	enterCalls  = []string{"SetMinSize 0x0", splashSize, "Center"}
	restoreFrom = func(w, h, x, y int) []string {
		return []string{minSize, "SetSize " + dims(w, h), "SetPosition " + dims(x, y)}
	}
)

func (r *splashRig) got(t *testing.T, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(r.win.calls, want) {
		t.Fatalf("window calls:\n got  %v\n want %v", r.win.calls, want)
	}
	r.win.calls = nil
}

func TestSplashEntersByDroppingTheMinimumResizingAndCentringOnceItLanded(t *testing.T) {
	r := newSplashRig()
	r.win.lag = 3
	r.splash.Enter("frangfurd")
	r.got(t, enterCalls...)
	if r.win.w != design.SplashWidth || r.win.h != design.SplashHeight {
		t.Fatalf("the window is %dx%d", r.win.w, r.win.h)
	}
	// Centring on the old size is the bug this order avoids: Center came
	// only after GetSize reported the new size, which took four looks.
	if r.win.centredAt < 5 {
		t.Fatalf("centred after %d size reads, before the resize landed", r.win.centredAt)
	}
	if r.sleeps != 3 {
		t.Fatalf("waited %d polls", r.sleeps)
	}
	if !r.splash.Showing("frangfurd") || r.splash.Showing("luxemburg") {
		t.Fatal("the card is up for the chapter that started")
	}
}

func TestSplashEntersAMaximisedWindowRememberingItsRestoredBounds(t *testing.T) {
	r := newSplashRig()
	r.win.maximised = true
	r.splash.Enter("frangfurd")
	r.got(t, append([]string{"Unmaximise"}, enterCalls...)...)

	r.splash.Handover("frangfurd")
	r.got(t, "Minimise")
	r.splash.Observe("frangfurd", models.GamePhaseClosed)
	// Back as it was: restored bounds first, maximised last.
	r.got(t, append(append([]string{"Unminimise"}, restoreFrom(1100, 700, 300, 120)...), "Maximise")...)
}

func TestSplashGivesUpWaitingForTheSizeAfterASecondAndCentresAnyway(t *testing.T) {
	logs := captureLog(t)
	r := newSplashRig()
	r.win.lag = 1 << 20
	r.splash.Enter("frangfurd")
	r.got(t, enterCalls...)
	if r.sleeps != 40 {
		t.Fatalf("polled every 25 ms for a second: %d waits", r.sleeps)
	}
	if !strings.Contains(logs.String(), "gave up waiting") {
		t.Fatalf("giving up is logged:\n%s", logs.String())
	}
}

// A window that scales the size it is given never reports the asked one: a
// size that changed and then stopped is taken as landed.
func TestSplashAcceptsASizeThatSettledOnSomethingElse(t *testing.T) {
	scaled := &scalingWindow{fakeWindow: newFakeWindow()}
	s := NewSplashWindow(scaled)
	sleeps := 0
	s.sleep = func(time.Duration) { sleeps++ }
	s.Enter("frangfurd")
	if sleeps > 3 {
		t.Fatalf("waited %d polls for a size that had settled", sleeps)
	}
	if scaled.centredAt == 0 {
		t.Fatal("centred")
	}
}

// scalingWindow lands every resize 50% larger, as a scaled display can.
type scalingWindow struct{ *fakeWindow }

func (s *scalingWindow) SetSize(w, h int) {
	s.record("SetSize " + dims(w, h))
	s.pendingW, s.pendingH, s.pending = w*3/2, h*3/2, true
}

func TestSplashMinimisesOnTheHandoverOnly(t *testing.T) {
	r := newSplashRig()
	r.splash.Enter("frangfurd")
	r.got(t, enterCalls...)

	r.splash.Handover("luxemburg")
	r.got(t)
	r.splash.Handover("frangfurd")
	r.got(t, "Minimise")
	r.splash.Handover("frangfurd")
	r.got(t)
}

func TestSplashLeavesInTheMeasuredOrderAfterTheHandover(t *testing.T) {
	for _, phase := range []string{models.GamePhaseClosed, models.GamePhaseCrashed} {
		t.Run(phase, func(t *testing.T) {
			r := newSplashRig()
			r.splash.Enter("frangfurd")
			r.splash.Handover("frangfurd")
			r.win.calls = nil

			if r.splash.Observe("frangfurd", phase) {
				t.Fatal("the event says the card is gone")
			}
			// Unminimise, then the minimum size, then the size, then the place:
			// the size before the minimum leaves the window at the default size.
			r.got(t, append([]string{"Unminimise"}, restoreFrom(1100, 700, 300, 120)...)...)
			if r.splash.Showing("frangfurd") {
				t.Fatal("the card is gone")
			}
		})
	}
}

func TestSplashKeepsTheCardWhenARunEndsBadlyBeforeTheHandover(t *testing.T) {
	for _, phase := range []string{models.GamePhaseCrashed, models.GamePhaseFailed} {
		t.Run(phase, func(t *testing.T) {
			r := newSplashRig()
			r.splash.Enter("frangfurd")
			r.win.calls = nil

			if !r.splash.Observe("frangfurd", phase) {
				t.Fatal("the event keeps the card, to show the error")
			}
			r.got(t)
			if !r.splash.Showing("frangfurd") {
				t.Fatal("still showing")
			}
			// A handover still on its way when the run ended does nothing.
			r.splash.Handover("frangfurd")
			r.got(t)
			// The player leaves the card.
			chapter, left := r.splash.Leave()
			if chapter != "frangfurd" || !left {
				t.Fatalf("%q %v", chapter, left)
			}
			r.got(t, restoreFrom(1100, 700, 300, 120)...)
		})
	}
}

func TestSplashLeavesWhenTheGameClosesBeforeTheHandover(t *testing.T) {
	r := newSplashRig()
	r.splash.Enter("frangfurd")
	r.win.calls = nil
	if r.splash.Observe("frangfurd", models.GamePhaseClosed) {
		t.Fatal("the card is gone")
	}
	// Never minimised, so nothing to unminimise.
	r.got(t, restoreFrom(1100, 700, 300, 120)...)
}

func TestSplashPhasesOtherThanAnEndChangeNothing(t *testing.T) {
	r := newSplashRig()
	r.splash.Enter("frangfurd")
	r.win.calls = nil
	for _, phase := range []string{"starting", "mods", "window", "resources", "running", "stopping"} {
		if !r.splash.Observe("frangfurd", phase) {
			t.Fatalf("%s still shows the card", phase)
		}
		if r.splash.Observe("luxemburg", phase) {
			t.Fatalf("%s: another chapter's event does not", phase)
		}
	}
	r.got(t)
}

func TestSplashLeftBeforeTheHandoverDoesNotMinimiseAtTheReload(t *testing.T) {
	r := newSplashRig()
	r.splash.Enter("frangfurd")
	r.win.calls = nil

	chapter, left := r.splash.Leave()
	if chapter != "frangfurd" || !left {
		t.Fatalf("%q %v", chapter, left)
	}
	r.got(t, restoreFrom(1100, 700, 300, 120)...)
	if r.splash.Showing("frangfurd") {
		t.Fatal("the card is gone")
	}

	// The game still appears at the reload, and the launcher stays.
	r.splash.Handover("frangfurd")
	if r.splash.Observe("frangfurd", models.GamePhaseResources) {
		t.Fatal("later events carry no splash")
	}
	if r.splash.Observe("frangfurd", models.GamePhaseClosed) {
		t.Fatal("nor the end")
	}
	r.got(t)
}

func TestSplashLeftAfterTheHandoverUnminimisesAtOnce(t *testing.T) {
	r := newSplashRig()
	r.splash.Enter("frangfurd")
	r.splash.Handover("frangfurd")
	r.win.calls = nil
	if _, left := r.splash.Leave(); !left {
		t.Fatal("a card was up")
	}
	r.got(t, append([]string{"Unminimise"}, restoreFrom(1100, 700, 300, 120)...)...)
	// The end of the run finds nothing to give back.
	r.splash.Observe("frangfurd", models.GamePhaseClosed)
	r.got(t)
}

func TestSplashWithNoCardUpDoesNothing(t *testing.T) {
	r := newSplashRig()
	if chapter, left := r.splash.Leave(); left || chapter != "" {
		t.Fatalf("%q %v", chapter, left)
	}
	r.splash.Handover("frangfurd")
	if r.splash.Observe("frangfurd", models.GamePhaseClosed) || r.splash.Observe("frangfurd", "running") {
		t.Fatal("no card, no flag")
	}
	if r.splash.Showing("frangfurd") {
		t.Fatal("no card")
	}
	r.got(t)
}

// A second Play while the card of a run that crashed is still up must not
// remember the card's shape as the player's own.
func TestSplashEnteredAgainKeepsTheFirstRememberedWindow(t *testing.T) {
	r := newSplashRig()
	r.splash.Enter("frangfurd")
	r.splash.Observe("frangfurd", models.GamePhaseCrashed)
	r.win.calls = nil

	r.splash.Enter("frangfurd")
	r.got(t)
	if !r.splash.Showing("frangfurd") {
		t.Fatal("the card is for the new run")
	}
	// The new run is a fresh one: it can be handed over.
	r.splash.Handover("frangfurd")
	r.got(t, "Minimise")
	r.splash.Observe("frangfurd", models.GamePhaseClosed)
	r.got(t, append([]string{"Unminimise"}, restoreFrom(1100, 700, 300, 120)...)...)
}

// With no window the sizes are zero: nothing is restored to zero.
func TestSplashDoesNotRestoreAWindowItNeverSaw(t *testing.T) {
	r := newSplashRig()
	r.win.w, r.win.h = 0, 0
	r.splash.Enter("frangfurd")
	r.win.calls = nil
	r.splash.Leave()
	r.got(t, minSize)
}
