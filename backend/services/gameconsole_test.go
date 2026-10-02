package services

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeConsole is a hold on Prism's console that records what was asked of it.
type fakeConsole struct {
	pid int
	mu  sync.Mutex
	// hides and held are what the hold says it hid and still has.
	hides, held int
	shows       int
	closes      int
	releases    int
}

func (f *fakeConsole) set(hides, held int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hides, f.held = hides, held
}

func (f *fakeConsole) Hides() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hides
}

func (f *fakeConsole) Held() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.held
}

func (f *fakeConsole) Show() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shows++
	return f.held > 0
}

func (f *fakeConsole) Close() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closes++
	return f.held
}

func (f *fakeConsole) Release() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releases++
}

func (f *fakeConsole) counts() (shows, closes, releases int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.shows, f.closes, f.releases
}

// fakeHoldConsole is the tracker's holdConsole function.
type fakeHoldConsole struct {
	mu      sync.Mutex
	err     error
	calls   int
	holders []*fakeConsole
}

func (f *fakeHoldConsole) hold(pid int) (ConsoleHolder, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	c := &fakeConsole{pid: pid}
	f.holders = append(f.holders, c)
	return c, nil
}

func (f *fakeHoldConsole) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeHoldConsole) holder(i int) *fakeConsole {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.holders[i]
}

func (f *fakeHoldConsole) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.holders)
}

// consoleRig is a rig whose game window, Prism's dialogs and Prism's console are
// all held by fakes, with a short wait for Prism to close.
func consoleRig(t *testing.T, hold bool) (*gameRig, *fakeHoldConsole) {
	t.Helper()
	r, _ := holdRig(t, hold)
	r.tracker.holdDialogs = (&fakeHoldDialogs{}).hold
	fc := &fakeHoldConsole{}
	r.tracker.holdConsole = fc.hold
	r.tracker.stopForce = 30 * time.Millisecond
	r.tracker.endWait = 20 * time.Millisecond
	return r, fc
}

// untilHeld waits for the run's hold to be on the tracker's record.
func untilHeld(t *testing.T, r *gameRig) {
	t.Helper()
	until(t, "the hold", func() bool { return r.tracker.console("frangfurd") != nil })
}

// failedByConsole runs a start to the console appearing and the run ending
// failed, and returns the run and its hold.
func failedByConsole(t *testing.T, r *gameRig, fc *fakeHoldConsole) (*gameRun, *fakeConsole) {
	t.Helper()
	run := r.begin()
	run.holdPrismConsole()
	if fc.count() != 1 {
		t.Fatalf("holds %d", fc.count())
	}
	c := fc.holder(0)
	c.set(1, 1)
	if r.stepAfter(run, time.Second) {
		t.Fatal("the run waits a moment for Prism's own log first")
	}
	if !r.stepAfter(run, 2*time.Second) {
		t.Fatalf("the console appearing during the start ends it: %s", run.state.Phase)
	}
	return run, c
}

func TestTrackerHoldsPrismsConsoleOnlyWhileTheSplashIsOn(t *testing.T) {
	r, fc := consoleRig(t, false)
	r.start()
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n")
	r.untilPhase("mods")
	time.Sleep(20 * time.Millisecond)
	if fc.callCount() != 0 || r.tracker.consoleAvailable("frangfurd") {
		t.Fatalf("held %d times with the splash off", fc.callCount())
	}

	r, fc = consoleRig(t, true)
	r.start()
	until(t, "the hold", func() bool { return fc.count() == 1 })
	if fc.holder(0).pid != 100 {
		t.Fatalf("held pid %d, want the launcher's own Prism (100)", fc.holder(0).pid)
	}
	if shows, closes, releases := fc.holder(0).counts(); shows+closes+releases != 0 {
		t.Fatalf("nothing is asked of it yet: %d %d %d", shows, closes, releases)
	}
}

func TestTrackerEndsAStartFailedWhenTheConsoleAppearsBeforeTheGameLog(t *testing.T) {
	r, fc := consoleRig(t, true)
	run, c := failedByConsole(t, r, fc)
	if run.state.Phase != "failed" || run.state.Reason != "launch" {
		t.Fatalf("failed, for a launch step: %+v", run.state)
	}
	if got := r.tracker.Latest("frangfurd"); got.Phase != "failed" || got.Reason != "launch" || r.tracker.Active("frangfurd") {
		t.Fatalf("the state held says so and Play is offered again: %+v", got)
	}
	// The run's end leaves the console alone: hidden, kept, Prism still up.
	if _, closes, releases := c.counts(); closes != 0 || releases != 0 {
		t.Fatalf("the hold outlives the run: closes %d, releases %d", closes, releases)
	}
	report, err := r.tracker.Report("frangfurd")
	if err != nil || !report.ConsoleAvailable {
		t.Fatalf("the report offers the console: %+v, %v", report, err)
	}
}

// Prism's own log, when it names the failure, ends the run with its reason, and
// that wins over the console, which only says that something failed.
func TestTrackerTakesTheReasonFromPrismsLogOverTheConsole(t *testing.T) {
	r, fc := consoleRig(t, true)
	plog := newPrismLog(t, r)
	run := r.begin()
	run.holdPrismConsole()
	fc.holder(0).set(1, 1)
	if r.stepAfter(run, time.Millisecond) {
		t.Fatal("nothing yet")
	}
	plog.write(prismSyncFailure)
	if !r.stepAfter(run, time.Millisecond) || run.state.Reason != "packsync" {
		t.Fatalf("the log's line is the reason: %+v", run.state)
	}
	if got := r.tracker.consoleAvailable("frangfurd"); !got {
		t.Fatal("the console is kept all the same")
	}
}

func TestTrackerLeavesAConsoleThatAppearsAfterTheGameLogBeganAlone(t *testing.T) {
	r, fc := consoleRig(t, true)
	run := r.begin()
	run.holdPrismConsole()
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n")
	if r.stepAfter(run, time.Second) || run.state.Phase != "mods" {
		t.Fatalf("the game log began: %s", run.state.Phase)
	}
	fc.holder(0).set(1, 1)
	for range 3 {
		if r.stepAfter(run, 5*time.Second) {
			t.Fatalf("a console the player opened is not a failure: %s", run.state.Phase)
		}
	}
	if run.state.Phase != "mods" {
		t.Fatalf("%s", run.state.Phase)
	}
}

// A console seen while the start waits is not a failure once the game's log
// begins within the grace: the game is going.
func TestTrackerDoesNotFailAStartWhoseGameLogBeganWithinTheConsoleGrace(t *testing.T) {
	r, fc := consoleRig(t, true)
	run := r.begin()
	run.holdPrismConsole()
	fc.holder(0).set(1, 1)
	if r.stepAfter(run, time.Millisecond) {
		t.Fatal("seen, not yet failed")
	}
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n")
	if r.stepAfter(run, 5*time.Second) || run.state.Phase != "mods" {
		t.Fatalf("the game log began first: %s", run.state.Phase)
	}
}

func TestTrackerKeepsAStoppedRunsReasonWhenTheConsoleAppears(t *testing.T) {
	r, fc := consoleRig(t, true)
	run := r.begin()
	run.holdPrismConsole()
	run.markStopped()
	fc.holder(0).set(1, 1)
	if r.stepAfter(run, 5*time.Second) || r.stepAfter(run, 5*time.Second) {
		t.Fatalf("the player's stop ends it, not the console: %+v", run.state)
	}
}

func TestTrackerGoesOnWhenPrismsConsoleCannotBeHeldAndSaysSoOnce(t *testing.T) {
	logs := captureLog(t)
	r, fc := consoleRig(t, true)
	fc.err = errors.New("no hook for you")
	r.start()
	until(t, "the attempt", func() bool { return fc.callCount() == 1 })
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n" + render + "Sound engine started\n")
	r.untilPhase("running")
	r.tracker.mu.Lock()
	delete(r.tracker.states, "frangfurd")
	r.tracker.mu.Unlock()
	r.start()
	until(t, "the second attempt", func() bool { return fc.callCount() == 2 })
	if n := strings.Count(logs.String(), "prism console cannot be held"); n != 1 {
		t.Fatalf("logged %d times:\n%s", n, logs.String())
	}
	if r.tracker.consoleAvailable("frangfurd") {
		t.Fatal("nothing is held")
	}
}

func TestShowConsoleShowsTheHeldConsoleAndSaysFalseWithNone(t *testing.T) {
	r, fc := consoleRig(t, true)
	if shown, err := r.tracker.ShowConsole("frangfurd"); shown || err != nil {
		t.Fatalf("nothing is held: %v, %v", shown, err)
	}
	_, c := failedByConsole(t, r, fc)

	shown, err := r.tracker.ShowConsole("frangfurd")
	if !shown || err != nil {
		t.Fatalf("shown %v, %v", shown, err)
	}
	if shows, _, _ := c.counts(); shows != 1 {
		t.Fatalf("the holder was asked to show once: %d", shows)
	}
	// Another chapter's console is not this one's.
	if shown, _ := r.tracker.ShowConsole("luxemburg"); shown {
		t.Fatal("no console for a chapter that never held one")
	}
	// Its windows closed by the player: none is left to show.
	c.set(1, 0)
	if shown, _ := r.tracker.ShowConsole("frangfurd"); shown || r.tracker.consoleAvailable("frangfurd") {
		t.Fatal("no window left")
	}
}

func TestTheConsoleIsNoLongerOfferedOnceItsPrismHasGone(t *testing.T) {
	r, fc := consoleRig(t, true)
	_, c := failedByConsole(t, r, fc)
	if !r.tracker.consoleAvailable("frangfurd") {
		t.Fatal("offered while Prism is up")
	}
	close(r.prism)
	until(t, "the hook to be let go", func() bool {
		_, _, releases := c.counts()
		return releases == 1
	})
	if r.tracker.consoleAvailable("frangfurd") || r.tracker.console("frangfurd") != nil {
		t.Fatal("a Prism that exited has no console to show")
	}
	if shown, _ := r.tracker.ShowConsole("frangfurd"); shown {
		t.Fatal("shown for a Prism that has gone")
	}
	report, err := r.tracker.Report("frangfurd")
	if err != nil || report.ConsoleAvailable {
		t.Fatalf("%+v, %v", report, err)
	}
}

// A Stop on a start whose console is hidden asks Prism to close through the
// held windows, which the OS routine (visible windows only) would skip.
func TestStopClosesAHiddenConsoleThroughTheHoldAndDoesNotEndPrismAtOnce(t *testing.T) {
	r, fc := consoleRig(t, true)
	r.procs.closeErr = errors.New("no visible window")
	r.start()
	untilHeld(t, r)
	fc.holder(0).set(1, 1)
	// Stop answers before the console is seen by a step, or after: either way
	// the hidden window is part of the close.
	if err := r.tracker.Stop("frangfurd"); err != nil {
		t.Fatal(err)
	}
	if _, closes, _ := fc.holder(0).counts(); closes != 1 {
		t.Fatalf("the held console was asked to close: %d", closes)
	}
	if got := r.procs.closed(); !reflect.DeepEqual(got, []int{100}) {
		t.Fatalf("the OS routine is asked as well: %v", got)
	}
	if got := r.procs.terminated(); len(got) != 0 {
		t.Fatalf("a Prism that took a close is not ended at once: %v", got)
	}
	close(r.prism)
	r.wantEnded("failed")
}

func TestStopWithNoConsoleHeldEndsAPrismThatTookNoCloseAtOnce(t *testing.T) {
	r, _ := consoleRig(t, true)
	r.procs.closeErr = errors.New("no visible window")
	r.start()
	untilHeld(t, r)
	if err := r.tracker.Stop("frangfurd"); err != nil {
		t.Fatal(err)
	}
	until(t, "Prism to be ended", func() bool { return len(r.procs.terminated()) > 0 })
	close(r.prism)
	r.wantEnded("failed")
}

// The next Play of the chapter closes the Prism the last start left on its
// console, and replaces the hold.
func TestTheNextPlayClosesThePrismOnTheOldConsoleAndReplacesTheHold(t *testing.T) {
	r, fc := consoleRig(t, true)
	_, old := failedByConsole(t, r, fc)
	oldPrism := r.prism
	r.prism = make(chan struct{})

	// The old Prism closes when asked.
	go func() {
		for len(r.procs.closed()) == 0 {
			time.Sleep(time.Millisecond)
		}
		close(oldPrism)
	}()
	r.start()
	until(t, "the second hold", func() bool {
		c := r.tracker.console("frangfurd")
		return fc.count() == 2 && c != nil && c.holder == ConsoleHolder(fc.holder(1))
	})
	if got := r.procs.closed(); !reflect.DeepEqual(got, []int{100}) {
		t.Fatalf("the old Prism is asked to close: %v", got)
	}
	if _, closes, releases := old.counts(); closes != 1 || releases != 1 {
		t.Fatalf("its windows were closed and its hook let go: closes %d, releases %d", closes, releases)
	}
	if got := r.procs.terminated(); len(got) != 0 {
		t.Fatalf("it closed in time: %v", got)
	}
}

func TestTheNextPlayEndsAPrismOnTheOldConsoleThatIgnoresTheClose(t *testing.T) {
	r, fc := consoleRig(t, true)
	failedByConsole(t, r, fc)
	r.prism = make(chan struct{})
	r.start()
	until(t, "the old Prism to be ended", func() bool { return len(r.procs.terminated()) > 0 })
	if got := r.procs.terminated(); !reflect.DeepEqual(got, []termCall{{100, true}}) {
		t.Fatalf("by its pid, forcibly, once: %v", got)
	}
}

func TestCloseConsoleLeavesAPrismWithNoConsoleLeftAlone(t *testing.T) {
	r, fc := consoleRig(t, true)
	_, c := failedByConsole(t, r, fc)
	c.set(1, 0) // the player closed it, or it never held a window
	r.tracker.CloseConsole("frangfurd")
	if got := r.procs.closed(); len(got) != 0 {
		t.Fatalf("a Prism with a game running is not the launcher's to close: %v", got)
	}
	if _, _, releases := c.counts(); releases != 1 {
		t.Fatalf("the hook is let go: %d", releases)
	}
	r.tracker.CloseConsole("frangfurd") // nothing left: a no-op
	r.tracker.CloseConsole("luxemburg")
}

func TestShutdownClosesEveryPrismLeftOnAConsoleAndReleasesTheHooks(t *testing.T) {
	r, fc := consoleRig(t, true)
	_, c := failedByConsole(t, r, fc)
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.tracker.Shutdown()
	}()
	// Prism closes by itself once asked.
	until(t, "the close request", func() bool { return len(r.procs.closed()) > 0 })
	close(r.prism)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not return")
	}
	if got := r.procs.closed(); !reflect.DeepEqual(got, []int{100}) {
		t.Fatalf("%v", got)
	}
	if _, closes, releases := c.counts(); closes != 1 || releases != 1 {
		t.Fatalf("closes %d, releases %d", closes, releases)
	}
	if got := r.procs.terminated(); len(got) != 0 {
		t.Fatalf("it closed in time: %v", got)
	}
	if r.tracker.consoleAvailable("frangfurd") {
		t.Fatal("nothing is held after quitting")
	}
}

// Quitting never hangs on a Prism that ignores the close: it is ended after
// stopForce, and Shutdown returns.
func TestShutdownEndsAPrismThatIgnoresTheCloseAndReturnsInTime(t *testing.T) {
	r, fc := consoleRig(t, true)
	failedByConsole(t, r, fc)
	start := time.Now()
	r.tracker.Shutdown()
	if got := r.procs.terminated(); !reflect.DeepEqual(got, []termCall{{100, true}}) {
		t.Fatalf("%v", got)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("took %s", elapsed)
	}
}

func TestShutdownWithNothingHeldDoesNothing(t *testing.T) {
	r, _ := consoleRig(t, true)
	r.tracker.Shutdown()
	if len(r.procs.closed()) != 0 || len(r.procs.terminated()) != 0 {
		t.Fatal("nothing to close")
	}
}

func TestIsPrismConsoleTitleMatchesOnlyTheConsole(t *testing.T) {
	cases := map[string]bool{
		"Console window for Frangfurd - Prism Launcher 11.1.1": true,
		"Console window for":                             true,
		"Console window for Lichdenstein":                true,
		"Please wait... - Prism Launcher 11.1.1":         false,
		"Sign in":                                        false,
		"Prism Launcher 11.1.1":                          false,
		"Error":                                          false,
		"Minecraft Console window for Frangfurd":         false,
		"console window for Frangfurd":                   false,
		" Console window for Frangfurd":                  false,
		"Konsolenfenster für Frangfurd - Prism Launcher": false, // a translated Prism matches nothing
		"": false,
	}
	for title, want := range cases {
		if got := isPrismConsoleTitle(title); got != want {
			t.Errorf("%q: got %v, want %v", title, got, want)
		}
		// The console is never taken for a progress dialog, nor one for the other.
		if isPrismDialogTitle(title) && isPrismConsoleTitle(title) {
			t.Errorf("%q matches both", title)
		}
	}
}
