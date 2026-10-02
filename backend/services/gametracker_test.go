package services

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"kapital/backend/models"
)

// fakeClock is the tracker's time: it moves only when a test moves it.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// fakeProcs is the OS's process table, with processes the test starts and
// ends. wait blocks on the pid's channel until the test ends it.
type fakeProcs struct {
	mu      sync.Mutex
	procs   []procInfo
	started map[int]time.Time
	exits   map[int]chan procExit
	listErr error
	lists   int
}

func newFakeProcs() *fakeProcs {
	return &fakeProcs{started: map[int]time.Time{}, exits: map[int]chan procExit{}}
}

func (f *fakeProcs) add(pid, ppid int, name string, created time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.procs = append(f.procs, procInfo{PID: pid, PPID: ppid, Name: name})
	f.started[pid] = created
}

func (f *fakeProcs) exitChan(pid int) chan procExit {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.exits[pid] == nil {
		f.exits[pid] = make(chan procExit, 1)
	}
	return f.exits[pid]
}

// end makes the process exit with a code.
func (f *fakeProcs) end(pid, code int) {
	f.exitChan(pid) <- procExit{code: code, known: true}
}

func (f *fakeProcs) os() gameOS {
	return gameOS{
		list: func() ([]procInfo, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.lists++
			return append([]procInfo(nil), f.procs...), f.listErr
		},
		started: func(pid int) (time.Time, bool) {
			f.mu.Lock()
			defer f.mu.Unlock()
			t, ok := f.started[pid]
			return t, ok
		},
		wait: func(ctx context.Context, pid int) (int, bool, error) {
			select {
			case ex := <-f.exitChan(pid):
				return ex.code, ex.known, ex.err
			case <-ctx.Done():
				return 0, false, ctx.Err()
			}
		},
	}
}

// gameRig is a tracker over a fake clock, fake processes and a temp folder.
type gameRig struct {
	t       *testing.T
	tracker *GameTracker
	clock   *fakeClock
	procs   *fakeProcs
	log     *gameLog
	play    time.Time
	prism   chan struct{}
	// hold is TrackRequest.HoldWindow for the requests the rig makes.
	hold bool
	// prismRoot is TrackRequest.PrismRoot for the requests the rig makes, ""
	// follows no Prism log.
	prismRoot string
	// onHandover is TrackRequest.OnHandover for the requests the rig makes.
	onHandover func()

	mu     sync.Mutex
	events []models.GameState
}

func newGameRig(t *testing.T) *gameRig {
	t.Helper()
	play := time.Now().Add(-time.Second)
	r := &gameRig{
		t:     t,
		clock: &fakeClock{now: play},
		procs: newFakeProcs(),
		log:   newGameLog(t),
		play:  play,
		prism: make(chan struct{}),
	}
	r.tracker = NewGameTracker(t.TempDir(), func(s models.GameState) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.events = append(r.events, s)
	})
	// Registered after the TempDir and before any start's cancel, so it runs
	// between them: every run has been cancelled, and none is still writing
	// its launch times when the directory is removed. Bounded, so a run that
	// ignores its cancel fails the test instead of hanging the package.
	t.Cleanup(func() {
		done := make(chan struct{})
		go func() {
			r.tracker.runs.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("a run did not end after its cancel")
		}
	})
	r.tracker.now = r.clock.Now
	r.tracker.os = r.procs.os()
	r.tracker.tick = time.Millisecond
	r.tracker.procInterval = 0
	r.tracker.procSlowInterval = 0
	return r
}

func (r *gameRig) request() TrackRequest {
	return TrackRequest{
		ChapterID:   "frangfurd",
		InstanceDir: r.log.dir,
		Prism:       PrismProcess{PID: 100, Exited: r.prism},
		PrismExe:    "/Prism/prismlauncher.exe",
		StartedAt:   r.play,
		Before:      SnapshotGameLog(r.log.dir),
		PrismRoot:   r.prismRoot,
		PrismLog:    SnapshotPrismLog(r.prismRoot),
		HoldWindow:  r.hold,
		OnHandover:  r.onHandover,
	}
}

// start follows the launch on its own goroutine, as the App does.
func (r *gameRig) start() {
	r.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	r.t.Cleanup(cancel)
	if err := r.tracker.Track(ctx, r.request()); err != nil {
		r.t.Fatal(err)
	}
}

// begin creates the run without a goroutine, so a test calls step itself.
func (r *gameRig) begin() *gameRun {
	r.t.Helper()
	run, err := r.tracker.begin(context.Background(), r.request())
	if err != nil {
		r.t.Fatal(err)
	}
	r.t.Cleanup(func() {
		if run.cancelWait != nil {
			run.cancelWait()
		}
	})
	return run
}

func (r *gameRig) phases() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, e := range r.events {
		out = append(out, e.Phase)
	}
	return out
}

// until waits, in real milliseconds, for a condition the tracker's goroutine
// brings about.
func until(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// untilPhase waits for the phase's event to have reached the rig. Latest
// shows a phase a moment before its event is emitted, so a test that waited
// on Latest and then read the events could miss the last one.
func (r *gameRig) untilPhase(phase string) {
	r.t.Helper()
	until(r.t, "phase "+phase, func() bool { return r.lastPhase() == phase })
}

func (r *gameRig) lastPhase() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.events) == 0 {
		return ""
	}
	return r.events[len(r.events)-1].Phase
}

// drive runs steps by hand until the condition holds, moving the clock by
// the given amount each time.
func (r *gameRig) drive(run *gameRun, advance time.Duration, what string, cond func() bool) {
	r.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			r.t.Fatalf("timed out waiting for %s (phase %s)", what, run.state.Phase)
		}
		r.clock.Advance(advance)
		if run.step(r.clock.Now()) {
			return
		}
		time.Sleep(time.Millisecond)
	}
}

func TestTrackerFollowsANormalRunToClosed(t *testing.T) {
	r := newGameRig(t)
	r.procs.add(100, 1, "prismlauncher.exe", r.play)
	r.procs.add(200, 100, "javaw.exe", r.play.Add(7*time.Second))
	r.start()
	if got := r.tracker.Latest("frangfurd"); got.Phase != "starting" || got.StartedAt == "" || !r.tracker.Active("frangfurd") {
		t.Fatalf("Play starts in starting: %+v", got)
	}

	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n")
	r.untilPhase("mods")
	r.clock.Advance(24 * time.Second)
	r.log.append(render + "Backend library: LWJGL\n")
	r.untilPhase("window")
	r.clock.Advance(16 * time.Second)
	r.log.append(render + "Reloading ResourceManager\n")
	r.untilPhase("resources")
	r.clock.Advance(11 * time.Second)
	r.log.append(render + "Sound engine started\n")
	r.untilPhase("running")
	r.log.append(render + "Stopping!\n")
	r.untilPhase("stopping")
	if !r.tracker.Active("frangfurd") {
		t.Fatal("stopping is still active")
	}
	r.procs.end(200, 0)
	r.untilPhase("closed")

	got := r.tracker.Latest("frangfurd")
	if got.ExitCode == nil || *got.ExitCode != 0 || r.tracker.Active("frangfurd") {
		t.Fatalf("closed carries the game's exit code: %+v", got)
	}
	want := []string{"starting", "mods", "window", "resources", "running", "stopping", "closed"}
	if !reflect.DeepEqual(r.phases(), want) {
		t.Fatalf("got %v want %v", r.phases(), want)
	}
	times := r.tracker.Durations("frangfurd")
	if len(times) != 1 || times[0].PhaseMs["window"] != 24000 || times[0].PhaseMs["resources"] != 40000 || times[0].PhaseMs["running"] != 51000 {
		t.Fatalf("the start's timings are kept from Play: %+v", times)
	}
}

func TestTrackerCallsAnExitBeforeStoppingACrash(t *testing.T) {
	r := newGameRig(t)
	r.procs.add(200, 100, "javaw.exe", r.play.Add(7*time.Second))
	r.start()
	r.log.write("[01:10:02] [main/INFO]: ModLauncher running\n" + render + "Backend library: LWJGL\n")
	r.untilPhase("window")
	r.procs.end(200, 1)
	r.untilPhase("crashed")
	if got := r.tracker.Latest("frangfurd"); got.ExitCode == nil || *got.ExitCode != 1 || r.tracker.Active("frangfurd") {
		t.Fatalf("%+v", got)
	}
	if len(r.tracker.Durations("frangfurd")) != 0 {
		t.Fatal("a start that never ran leaves no timings")
	}
}

func TestTrackerReadsTheLastLinesWrittenJustBeforeTheExit(t *testing.T) {
	r := newGameRig(t)
	r.procs.add(200, 100, "javaw.exe", r.play)
	run := r.begin()
	r.log.write("first\n")
	r.drive(run, time.Millisecond, "the game bound", func() bool { return run.bound == 200 })
	// "Stopping!" is written and the process ends within one step.
	r.log.append(render + "Stopping!\n")
	r.procs.end(200, 0)
	r.drive(run, time.Millisecond, "closed", func() bool { return run.state.Phase == "closed" })
	if run.state.Phase != "closed" {
		t.Fatalf("got %s", run.state.Phase)
	}
}

func TestTrackerFailsWhenNoLogComesInTime(t *testing.T) {
	r := newGameRig(t)
	r.tracker.startTimeout = 10 * time.Minute
	run := r.begin()
	r.clock.Advance(9 * time.Minute)
	if run.step(r.clock.Now()) || run.state.Phase != "starting" {
		t.Fatalf("still within the time: %s", run.state.Phase)
	}
	r.clock.Advance(2 * time.Minute)
	if !run.step(r.clock.Now()) || run.state.Phase != "failed" {
		t.Fatalf("no game log in ten minutes: %s", run.state.Phase)
	}
	if r.tracker.Active("frangfurd") {
		t.Fatal("failed is not active")
	}
}

func TestTrackerFailsWhenPrismExitsWithoutAGame(t *testing.T) {
	r := newGameRig(t)
	r.tracker.prismGrace = 30 * time.Second
	run := r.begin()
	close(r.prism)
	r.clock.Advance(5 * time.Second)
	if run.step(r.clock.Now()) {
		t.Fatal("Prism has only just exited")
	}
	r.clock.Advance(20 * time.Second)
	if run.step(r.clock.Now()) {
		t.Fatalf("within the grace period: %s", run.state.Phase)
	}
	r.clock.Advance(11 * time.Second)
	if !run.step(r.clock.Now()) || run.state.Phase != "failed" {
		t.Fatalf("Prism gone, no game and no log for 30 s: %s", run.state.Phase)
	}
}

func TestTrackerDoesNotEndWhenTheLogBeganAfterPrismExited(t *testing.T) {
	// The launcher's Prism handed the launch to one already open and exited
	// before any game log: the game is that other Prism's, and is waited for.
	r := newGameRig(t)
	r.tracker.prismGrace = 30 * time.Second
	run := r.begin()
	close(r.prism)
	if run.step(r.clock.Now()) {
		t.Fatal("Prism has only just exited")
	}
	r.log.write("first\n")
	r.clock.Advance(time.Minute)
	if run.step(r.clock.Now()) || run.state.Phase != "mods" {
		t.Fatalf("a fresh log is a game: %s", run.state.Phase)
	}
	r.clock.Advance(time.Hour)
	if run.step(r.clock.Now()) {
		t.Fatal("a Prism that went before the log does not end the game")
	}
}

// QuitAfterGameStop: the launcher's Prism quits with the game, so its exit
// after the log began ends a run that has no process to wait on.
func TestTrackerEndsWhenPrismQuitsAfterTheGameLogBegan(t *testing.T) {
	for _, c := range []struct {
		name    string
		log     string
		want    string
		stopped bool
	}{
		{"closed after Stopping!", "first\n" + render + "Backend library: LWJGL\n" + render + "Stopping!\n", "closed", true},
		{"crashed without it", "first\n" + render + "Backend library: LWJGL\n", "crashed", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newGameRig(t)
			run := r.begin()
			r.log.write(c.log)
			r.drive(run, 0, "the log read", func() bool { return run.state.Phase != "starting" })
			close(r.prism)
			r.clock.Advance(time.Second)
			if run.step(r.clock.Now()) {
				t.Fatal("given a moment first")
			}
			// Prism's last word is in the log before it goes.
			if !c.stopped {
				r.log.append(render + "Reloading ResourceManager\n")
			}
			r.clock.Advance(2 * time.Second)
			if !run.step(r.clock.Now()) || run.state.Phase != c.want || run.state.ExitCode != nil {
				t.Fatalf("got %+v, want %s", run.state, c.want)
			}
			if r.tracker.Active("frangfurd") {
				t.Fatal("the run is over")
			}
		})
	}
}

func TestTrackerDoesNotEndOnPrismQuittingWhileItWaitsOnTheGame(t *testing.T) {
	r := newGameRig(t)
	r.procs.add(200, 100, "javaw.exe", r.play)
	run := r.begin()
	run.step(r.clock.Now())
	r.log.write("first\n")
	r.drive(run, 0, "the log read", func() bool { return run.state.Phase == "mods" })
	close(r.prism)
	r.clock.Advance(time.Minute)
	if run.step(r.clock.Now()) {
		t.Fatal("the game's process, not Prism, ends a watched game")
	}
}

// A log that gets ahead of the lookup still gets a process.
func TestTrackerKeepsLookingForTheGameAfterTheLogRacesAhead(t *testing.T) {
	r := newGameRig(t)
	run := r.begin()
	r.log.write("first\n" + render + "Backend library: LWJGL\n")
	r.drive(run, 0, "window", func() bool { return run.state.Phase == "window" })
	if run.bound != 0 {
		t.Fatal("no Java yet")
	}
	r.procs.add(200, 100, "javaw.exe", r.play.Add(5*time.Second))
	r.drive(run, 0, "the game bound", func() bool { return run.bound == 200 })
	r.log.append(render + "Stopping!\n")
	r.procs.end(200, 0)
	r.drive(run, 0, "closed", func() bool { return run.state.Phase == "closed" })
	if run.state.ExitCode == nil || *run.state.ExitCode != 0 {
		t.Fatalf("the process ended it: %+v", run.state)
	}
}

func TestTrackerLooksLessOftenOnceTheLoaderIsPastMods(t *testing.T) {
	r := newGameRig(t)
	r.tracker.procInterval = time.Second
	r.tracker.procSlowInterval = 5 * time.Second
	run := r.begin()
	r.log.write("first\n" + render + "Backend library: LWJGL\n")
	r.clock.Advance(time.Second)
	run.step(r.clock.Now())
	if run.state.Phase != "window" {
		t.Fatalf("got %s", run.state.Phase)
	}
	calls := func() int {
		r.procs.mu.Lock()
		defer r.procs.mu.Unlock()
		return r.procs.lists
	}
	before := calls()
	for range 4 {
		r.clock.Advance(time.Second)
		run.step(r.clock.Now())
	}
	if calls() != before {
		t.Fatalf("looked %d more times within the slow interval", calls()-before)
	}
	r.clock.Advance(time.Second)
	run.step(r.clock.Now())
	if calls() != before+1 {
		t.Fatalf("looked %d more times after it", calls()-before)
	}
}

func TestTrackerSetsAsideAJavaThatEndsBeforeTheGameLog(t *testing.T) {
	// Prism's pre-launch command runs a Java (packwiz) that is a child of
	// Prism too, and ends before the game starts.
	r := newGameRig(t)
	r.procs.add(150, 100, "javaw.exe", r.play.Add(time.Second))
	run := r.begin()
	r.drive(run, time.Millisecond, "packwiz bound", func() bool { return run.bound == 150 })
	r.procs.end(150, 0)
	r.drive(run, time.Millisecond, "packwiz set aside", func() bool { return run.ignored[150] })
	if run.state.Phase != "starting" || run.bound != 0 {
		t.Fatalf("packwiz's exit is not the game's: %s", run.state.Phase)
	}
	r.procs.add(200, 100, "javaw.exe", r.play.Add(20*time.Second))
	r.log.write("first\n")
	r.drive(run, time.Millisecond, "the game bound", func() bool { return run.bound == 200 })
	r.log.append(render + "Stopping!\n")
	r.procs.end(200, 0)
	r.drive(run, time.Millisecond, "closed", func() bool { return run.state.Phase == "closed" })
}

func TestTrackerIgnoresAJavaThatWasAlreadyRunning(t *testing.T) {
	r := newGameRig(t)
	r.procs.add(150, 100, "javaw.exe", r.play.Add(-time.Hour))
	r.procs.add(160, 999, "javaw.exe", r.play.Add(time.Second))
	run := r.begin()
	for range 5 {
		r.clock.Advance(time.Millisecond)
		run.step(r.clock.Now())
	}
	if run.bound != 0 {
		t.Fatalf("a Java from before Play, or another process's child, is not the game: %d", run.bound)
	}
}

func TestTrackerTakesTheNewestJavaOfSeveral(t *testing.T) {
	r := newGameRig(t)
	r.procs.add(150, 100, "javaw.exe", r.play.Add(time.Second))
	r.procs.add(200, 100, "javaw.exe", r.play.Add(20*time.Second))
	run := r.begin()
	run.step(r.clock.Now())
	if run.bound != 200 {
		t.Fatalf("got %d", run.bound)
	}
}

// [verify] Play while Prism was already open: the launcher's Prism forwards
// and exits, and the game's Java is a child of the Prism that was open.
func TestTrackerFindsTheGameUnderAnotherPrismWhenOursHasExited(t *testing.T) {
	r := newGameRig(t)
	r.procs.add(300, 1, "prismlauncher.exe", r.play.Add(-time.Hour))
	r.procs.add(400, 300, "javaw.exe", r.play.Add(10*time.Second))
	run := r.begin()
	run.step(r.clock.Now())
	if run.bound != 0 {
		t.Fatalf("another Prism's child is only looked at once ours has exited: %d", run.bound)
	}
	close(r.prism)
	r.drive(run, time.Millisecond, "the game bound", func() bool { return run.bound == 400 })
	r.log.write("first\n")
	r.log.append(render + "Stopping!\n")
	r.procs.end(400, 0)
	r.drive(run, time.Millisecond, "closed", func() bool { return run.state.Phase == "closed" })
}

func TestTrackerFollowsTheLogAloneWhenProcessesCannotBeListed(t *testing.T) {
	r := newGameRig(t)
	r.procs.listErr = errGameProcUnsupported
	r.tracker.quietTimeout = 10 * time.Second
	run := r.begin()
	run.step(r.clock.Now())
	if !run.procUnavailable {
		t.Fatal("the lookup's failure is remembered")
	}
	r.log.write("first\n" + render + "Backend library: LWJGL\n")
	r.drive(run, 0, "window", func() bool { return run.state.Phase == "window" })
	r.log.append(render + "Stopping!\n")
	r.drive(run, 0, "stopping", func() bool { return run.state.Phase == "stopping" })
	r.clock.Advance(9 * time.Second)
	if run.step(r.clock.Now()) {
		t.Fatal("the log has only just gone quiet")
	}
	r.clock.Advance(2 * time.Second)
	if !run.step(r.clock.Now()) || run.state.Phase != "closed" || run.state.ExitCode != nil {
		t.Fatalf("Stopping! and ten quiet seconds close a game with no process: %+v", run.state)
	}
}

func TestTrackerDoesNotCloseOnAQuietLogWhileItWaitsOnTheGame(t *testing.T) {
	r := newGameRig(t)
	r.tracker.quietTimeout = 10 * time.Second
	r.procs.add(200, 100, "javaw.exe", r.play)
	run := r.begin()
	run.step(r.clock.Now())
	if run.bound != 200 {
		t.Fatalf("the game is found while the start is waited on: %d", run.bound)
	}
	r.log.write("first\n" + render + "Stopping!\n")
	r.drive(run, 0, "stopping", func() bool { return run.state.Phase == "stopping" })
	r.clock.Advance(time.Hour)
	if run.step(r.clock.Now()) || run.state.Phase != "stopping" {
		t.Fatalf("the process, not the log, ends a game that is watched: %s", run.state.Phase)
	}
}

func TestTrackerStartsOverWhenTheLogIsWrittenAgain(t *testing.T) {
	r := newGameRig(t)
	run := r.begin()
	r.log.write("[00:30:00] [main/INFO]: a\n" + render + "Backend library: LWJGL\n" + render + "Stopping!\n")
	r.drive(run, 0, "stopping", func() bool { return run.state.Phase == "stopping" })
	r.log.write("[00:50:00] [main/INFO]: b\n")
	r.drive(run, 0, "mods again", func() bool { return run.state.Phase == "mods" })
	if run.stopped {
		t.Fatal("the new run has not stopped")
	}
}

func TestTrackerRefusesASecondTrackWhileActiveAndAllowsOneAfterTheEnd(t *testing.T) {
	r := newGameRig(t)
	r.tracker.startTimeout = time.Minute
	run := r.begin()
	if _, err := r.tracker.begin(context.Background(), r.request()); err == nil || !strings.Contains(err.Error(), "already starting or running") {
		t.Fatalf("got %v", err)
	}
	if err := r.tracker.Track(context.Background(), r.request()); err == nil {
		t.Fatal("Track refuses too")
	}
	// Another chapter is its own.
	other := r.request()
	other.ChapterID = "lichdenstein"
	if _, err := r.tracker.begin(context.Background(), other); err != nil {
		t.Fatalf("a different chapter is free: %v", err)
	}
	r.clock.Advance(2 * time.Minute)
	if !run.step(r.clock.Now()) {
		t.Fatal("timed out")
	}
	if _, err := r.tracker.begin(context.Background(), r.request()); err != nil {
		t.Fatalf("a chapter whose game has ended can be launched again: %v", err)
	}
	all := r.tracker.All()
	if len(all) != 2 || all[0].ChapterID != "frangfurd" || all[1].ChapterID != "lichdenstein" {
		t.Fatalf("All is by chapter: %+v", all)
	}
}

func TestTrackerLatestIsIdleForAChapterNeverLaunched(t *testing.T) {
	r := newGameRig(t)
	if got := r.tracker.Latest("atlantis"); got.Phase != "idle" || got.ChapterID != "atlantis" || r.tracker.Active("atlantis") {
		t.Fatalf("%+v", got)
	}
}

func TestTrackerStopsWithItsContext(t *testing.T) {
	r := newGameRig(t)
	ctx, cancel := context.WithCancel(context.Background())
	if err := r.tracker.Track(ctx, r.request()); err != nil {
		t.Fatal(err)
	}
	cancel()
	time.Sleep(20 * time.Millisecond)
	before := len(r.phases())
	r.log.write("first\n")
	time.Sleep(20 * time.Millisecond)
	if len(r.phases()) != before {
		t.Fatal("a cancelled tracker does not go on reading")
	}
}

func TestRecordTimingKeepsTheLastFiveWithPrivateMode(t *testing.T) {
	r := newGameRig(t)
	for i := range 7 {
		r.tracker.recordTiming("frangfurd", models.LaunchTiming{StartedAt: time.Unix(int64(i), 0).UTC().Format(time.RFC3339), PhaseMs: map[string]int64{"running": int64(i)}})
	}
	r.tracker.recordTiming("lichdenstein", models.LaunchTiming{PhaseMs: map[string]int64{"running": 99}})
	got := r.tracker.Durations("frangfurd")
	if len(got) != 5 || got[0].PhaseMs["running"] != 2 || got[4].PhaseMs["running"] != 6 {
		t.Fatalf("the last five, oldest first: %+v", got)
	}
	if other := r.tracker.Durations("lichdenstein"); len(other) != 1 {
		t.Fatalf("each chapter keeps its own: %+v", other)
	}
	path := filepath.Join(r.tracker.dataDir, launchTimesFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk map[string][]models.LaunchTiming
	if err := json.Unmarshal(raw, &onDisk); err != nil || len(onDisk["frangfurd"]) != 5 {
		t.Fatalf("%v %s", err, raw)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("the file is private: %v", info.Mode())
	}
}

func TestRecordTimingStartsOverFromAFileItCannotRead(t *testing.T) {
	r := newGameRig(t)
	if err := os.WriteFile(filepath.Join(r.tracker.dataDir, launchTimesFile), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.tracker.recordTiming("frangfurd", models.LaunchTiming{PhaseMs: map[string]int64{"running": 1}})
	if got := r.tracker.Durations("frangfurd"); len(got) != 1 {
		t.Fatalf("%+v", got)
	}
}

func TestProcessNameHelpers(t *testing.T) {
	for name, want := range map[string]bool{"javaw.exe": true, "JAVA.EXE": true, "java": true, "javac": false, "prismlauncher.exe": false} {
		if isJavaName(name) != want {
			t.Fatalf("%s: want %v", name, want)
		}
	}
	cases := []struct {
		name, exe string
		want      bool
	}{
		{"prismlauncher.exe", "/Users/p/Prism/prismlauncher.exe", true},
		{"PrismLauncher.exe", `/x/prismlauncher.exe`, true},
		{"prismlauncher", "/Applications/Prism Launcher.app/Contents/MacOS/prismlauncher", true},
		{"prismlauncher", "", false},
		{"notepad.exe", "/x/prismlauncher.exe", false},
	}
	for _, c := range cases {
		if got := sameExe(c.name, c.exe); got != c.want {
			t.Fatalf("%s vs %s: got %v", c.name, c.exe, got)
		}
	}
}
