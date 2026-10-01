package services

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeDialogs records how a hold on Prism's dialogs was released.
type fakeDialogs struct {
	pid int
	mu  sync.Mutex
	got []bool
}

func (f *fakeDialogs) Release(show bool) DialogReport {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, show)
	return DialogReport{Hides: 4, ShownBack: show}
}

func (f *fakeDialogs) releases() []bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]bool(nil), f.got...)
}

// fakeHoldDialogs is the tracker's holdDialogs function.
type fakeHoldDialogs struct {
	mu      sync.Mutex
	err     error
	calls   int
	holders []*fakeDialogs
}

func (f *fakeHoldDialogs) hold(pid int) (DialogHolder, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	d := &fakeDialogs{pid: pid}
	f.holders = append(f.holders, d)
	return d, nil
}

func (f *fakeHoldDialogs) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.holders)
}

func (f *fakeHoldDialogs) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeHoldDialogs) holder(i int) *fakeDialogs {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.holders[i]
}

// dialogRig is a rig whose game window and Prism dialogs are held by fakes.
func dialogRig(t *testing.T, hold bool) (*gameRig, *fakeHoldDialogs) {
	t.Helper()
	r, _ := holdRig(t, hold)
	fd := &fakeHoldDialogs{}
	r.tracker.holdDialogs = fd.hold
	return r, fd
}

func TestTrackerHoldsPrismsDialogsOnlyWhileTheSplashIsOn(t *testing.T) {
	r, fd := dialogRig(t, false)
	r.start()
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n")
	r.untilPhase("mods")
	time.Sleep(20 * time.Millisecond)
	if fd.callCount() != 0 {
		t.Fatalf("held %d times with the splash off", fd.callCount())
	}

	r, fd = dialogRig(t, true)
	r.start()
	until(t, "the hold", func() bool { return fd.count() == 1 })
	if fd.holder(0).pid != 100 {
		t.Fatalf("held pid %d, want the launcher's own Prism (100)", fd.holder(0).pid)
	}
	if got := fd.holder(0).releases(); len(got) != 0 {
		t.Fatalf("released before anything happened: %v", got)
	}
}

func TestTrackerStartsHoldingPrismsDialogsBeforeTheGameIsFound(t *testing.T) {
	// Prism's first dialog is about a second in; the game's Java is minutes off.
	r, fd := dialogRig(t, true)
	run := r.begin()
	run.holdPrismDialogs()
	if fd.count() != 1 || run.bound != 0 {
		t.Fatalf("holds %d, bound %d", fd.count(), run.bound)
	}
	run.holdPrismDialogs()
	if fd.callCount() != 1 {
		t.Fatalf("one hold for one run, got %d", fd.callCount())
	}
}

func TestTrackerReleasesPrismsDialogsWithoutShowingThemAtTheHandover(t *testing.T) {
	logs := captureLog(t)
	r, fd := dialogRig(t, true)
	r.procs.add(200, 100, "javaw.exe", r.play.Add(7*time.Second))
	r.start()
	until(t, "the hold", func() bool { return fd.count() == 1 })
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n")
	r.untilPhase("mods")
	r.log.append(render + "Backend library: LWJGL\n")
	r.untilPhase("window")
	time.Sleep(20 * time.Millisecond)
	if got := fd.holder(0).releases(); len(got) != 0 {
		t.Fatalf("released before the handover: %v", got)
	}

	r.log.append(render + "Reloading ResourceManager\n")
	r.untilPhase("resources")
	until(t, "the release", func() bool { return len(fd.holder(0).releases()) == 1 })
	if got := fd.holder(0).releases(); !reflect.DeepEqual(got, []bool{false}) {
		t.Fatalf("the handover leaves the dialogs as they are: %v", got)
	}

	// Nothing more to release however the run ends.
	r.log.append(render + "Stopping!\n")
	r.untilPhase("stopping")
	r.procs.end(200, 0)
	r.untilPhase("closed")
	if got := fd.holder(0).releases(); len(got) != 1 {
		t.Fatalf("released again: %v", got)
	}

	until(t, "the log line", func() bool { return strings.Contains(logs.String(), `msg="prism dialogs"`) })
	line := logs.String()
	for _, want := range []string{"chapter=frangfurd", "hides=4", "shownBack=false"} {
		if !strings.Contains(line, want) {
			t.Errorf("log lacks %q:\n%s", want, line)
		}
	}
	if n := strings.Count(line, `msg="prism dialogs"`); n != 1 {
		t.Fatalf("one line per release, got %d:\n%s", n, line)
	}
}

func TestTrackerReleasesPrismsDialogsAtRunningWhenTheLogSkippedTheReload(t *testing.T) {
	r, fd := dialogRig(t, true)
	r.procs.add(200, 100, "javaw.exe", r.play.Add(7*time.Second))
	r.start()
	until(t, "the hold", func() bool { return fd.count() == 1 })
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n" + render + "Sound engine started\n")
	r.untilPhase("running")
	until(t, "the release", func() bool { return len(fd.holder(0).releases()) == 1 })
	if got := fd.holder(0).releases(); !reflect.DeepEqual(got, []bool{false}) {
		t.Fatalf("got %v", got)
	}
}

func TestTrackerShowsPrismsDialogsBackWhenTheRunEndsBeforeTheHandover(t *testing.T) {
	endings := map[string]func(r *gameRig, run *gameRun){
		"crashed": func(r *gameRig, run *gameRun) {
			r.procs.end(200, 1)
			r.drive(run, time.Millisecond, "crashed", func() bool { return run.state.Phase == "crashed" })
		},
		"closed": func(r *gameRig, run *gameRun) {
			r.log.append(render + "Stopping!\n")
			r.procs.end(200, 0)
			r.drive(run, time.Millisecond, "closed", func() bool { return run.state.Phase == "closed" })
		},
		"stopping": func(r *gameRig, run *gameRun) {
			r.log.append(render + "Stopping!\n")
			r.drive(run, time.Millisecond, "stopping", func() bool { return run.state.Phase == "stopping" })
		},
		"failed": func(r *gameRig, run *gameRun) {
			run.set("failed", r.clock.Now(), nil)
		},
		"failed by the clock": func(r *gameRig, run *gameRun) {
			// No game log ever comes: the start times out.
			r.clock.Advance(r.tracker.startTimeout)
			r.drive(run, time.Millisecond, "failed", func() bool { return run.state.Phase == "failed" })
		},
	}
	for name, end := range endings {
		t.Run(name, func(t *testing.T) {
			logs := captureLog(t)
			r, fd := dialogRig(t, true)
			r.procs.add(200, 100, "javaw.exe", r.play.Add(7*time.Second))
			run := r.begin()
			run.holdPrismDialogs()
			if name != "failed by the clock" {
				r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n")
			}
			r.drive(run, time.Millisecond, "the hold", func() bool { return fd.count() == 1 })
			end(r, run)
			if got := fd.holder(0).releases(); !reflect.DeepEqual(got, []bool{true}) {
				t.Fatalf("an error Prism is showing must not stay hidden: %v", got)
			}
			if !strings.Contains(logs.String(), "shownBack=true") {
				t.Fatalf("the log says whether any was shown back:\n%s", logs.String())
			}
		})
	}
}

func TestTrackerShowsPrismsDialogsBackWhenItsContextEnds(t *testing.T) {
	r, fd := dialogRig(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	if err := r.tracker.Track(ctx, r.request()); err != nil {
		t.Fatal(err)
	}
	until(t, "the hold", func() bool { return fd.count() == 1 })
	cancel()
	until(t, "the release", func() bool { return len(fd.holder(0).releases()) > 0 })
	if got := fd.holder(0).releases(); !reflect.DeepEqual(got, []bool{true}) {
		t.Fatalf("got %v", got)
	}
}

func TestTrackerDoesNotHoldTheDialogsOfAPrismItHandedTheLaunchTo(t *testing.T) {
	// Play while Prism was already open: the launcher's Prism exits at once.
	// What it showed is gone with it, and the other Prism's dialogs are not
	// held. [verify] on a real start (#95).
	r, fd := dialogRig(t, true)
	r.start()
	until(t, "the hold", func() bool { return fd.count() == 1 })
	close(r.prism)
	until(t, "the release", func() bool { return len(fd.holder(0).releases()) > 0 })
	if got := fd.holder(0).releases(); !reflect.DeepEqual(got, []bool{true}) {
		t.Fatalf("got %v", got)
	}
	if fd.callCount() != 1 {
		t.Fatalf("no second hold on the other Prism: %d calls", fd.callCount())
	}
}

func TestTrackerGoesOnWhenPrismsDialogsCannotBeHeldAndSaysSoOnce(t *testing.T) {
	logs := captureLog(t)
	r, fd := dialogRig(t, true)
	fd.err = errors.New("no hook for you")
	r.start()
	until(t, "the attempt", func() bool { return fd.callCount() == 1 })
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n" + render + "Sound engine started\n")
	r.untilPhase("running")
	// A second run on the same tracker does not say it again.
	r.tracker.mu.Lock()
	delete(r.tracker.states, "frangfurd")
	r.tracker.mu.Unlock()
	r.start()
	until(t, "the second attempt", func() bool { return fd.callCount() == 2 })
	if n := strings.Count(logs.String(), "prism dialogs cannot be held"); n != 1 {
		t.Fatalf("logged %d times:\n%s", n, logs.String())
	}
}

func TestIsPrismDialogTitleMatchesOnlyTheProgressDialogs(t *testing.T) {
	cases := map[string]bool{
		"Please wait... - Prism Launcher 11.1.1": true,
		"Please wait":                            true,
		"Please wait...":                         true,
		"Sign in":                                false,
		"Prism Launcher 11.1.1":                  false,
		"Error":                                  false,
		"Minecraft account sign in - Please wait": false,
		"please wait":                      false,
		"Bitte warten... - Prism Launcher": false, // a translated Prism matches nothing
		"":                                 false,
	}
	for title, want := range cases {
		if got := isPrismDialogTitle(title); got != want {
			t.Errorf("%q: got %v, want %v", title, got, want)
		}
	}
}
