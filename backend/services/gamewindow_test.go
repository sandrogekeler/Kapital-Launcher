package services

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeHolder records how it was released.
type fakeHolder struct {
	pid int
	mu  sync.Mutex
	got []bool
}

func (f *fakeHolder) Release(foreground bool) WindowReport {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, foreground)
	return WindowReport{Seen: true, Hides: 2, FirstHideMs: 12, MaxHideMs: 30, Foreground: foreground}
}

func (f *fakeHolder) releases() []bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]bool(nil), f.got...)
}

// fakeHold is the tracker's hold function: it hands out fake holders, or an
// error.
type fakeHold struct {
	mu      sync.Mutex
	err     error
	calls   int
	holders []*fakeHolder
}

func (f *fakeHold) hold(pid int) (WindowHolder, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	h := &fakeHolder{pid: pid}
	f.holders = append(f.holders, h)
	return h, nil
}

func (f *fakeHold) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeHold) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.holders)
}

func (f *fakeHold) holder(i int) *fakeHolder {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.holders[i]
}

// lockedBuffer is a log sink two goroutines can use.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// captureLog points the default logger at a buffer until the test ends.
func captureLog(t *testing.T) *lockedBuffer {
	t.Helper()
	buf := &lockedBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

func holdRig(t *testing.T, hold bool) (*gameRig, *fakeHold) {
	t.Helper()
	r := newGameRig(t)
	fh := &fakeHold{}
	r.tracker.hold = fh.hold
	r.hold = hold
	return r, fh
}

func TestTrackerHoldsTheGameWindowUntilRunningThenHandsOver(t *testing.T) {
	logs := captureLog(t)
	r, fh := holdRig(t, true)
	r.procs.add(200, 100, "javaw.exe", r.play.Add(7*time.Second))
	r.start()

	until(t, "the hold", func() bool { return fh.count() == 1 })
	if fh.holder(0).pid != 200 {
		t.Fatalf("held pid %d, want the game's Java", fh.holder(0).pid)
	}
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n")
	r.untilPhase("mods")
	r.log.append(render + "Backend library: LWJGL\n")
	r.untilPhase("window")
	r.log.append(render + "Reloading ResourceManager\n")
	r.untilPhase("resources")
	if got := fh.holder(0).releases(); len(got) != 0 {
		t.Fatalf("released before the game was ready: %v", got)
	}

	r.log.append(render + "Sound engine started\n")
	r.untilPhase("running")
	if got := fh.holder(0).releases(); !reflect.DeepEqual(got, []bool{true}) {
		t.Fatalf("the handover shows the window with the foreground, once: %v", got)
	}

	r.log.append(render + "Stopping!\n")
	r.untilPhase("stopping")
	r.procs.end(200, 0)
	r.untilPhase("closed")
	if got := fh.holder(0).releases(); len(got) != 1 {
		t.Fatalf("nothing more to release after the handover: %v", got)
	}
	if fh.count() != 1 {
		t.Fatalf("one hold for one game, got %d", fh.count())
	}

	line := logs.String()
	for _, want := range []string{
		`msg="game window"`, "chapter=frangfurd", "handover=true", "seen=true", "hides=2",
		"firstHideMs=12", "maxHideMs=30", "foreground=true",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("log lacks %q:\n%s", want, line)
		}
	}
}

func TestTrackerNeverHoldsWhenTheSettingIsOff(t *testing.T) {
	r, fh := holdRig(t, false)
	r.procs.add(200, 100, "javaw.exe", r.play.Add(7*time.Second))
	r.start()
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n")
	r.untilPhase("mods")
	r.log.append(render + "Sound engine started\n")
	r.untilPhase("running")
	// Give the bound game's lookup every chance to have started a hold.
	time.Sleep(20 * time.Millisecond)
	if fh.callCount() != 0 {
		t.Fatalf("held %d times with the setting off", fh.callCount())
	}
}

func TestTrackerHoldsOnlyOnceTheGamesJavaIsBound(t *testing.T) {
	r, fh := holdRig(t, true)
	r.start()
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n")
	r.untilPhase("mods")
	time.Sleep(20 * time.Millisecond)
	if fh.count() != 0 {
		t.Fatal("no game process to hold yet")
	}
	r.procs.add(200, 100, "javaw.exe", r.play.Add(7*time.Second))
	until(t, "the hold", func() bool { return fh.count() == 1 })
}

func TestTrackerDoesNotHoldAGameThatIsAlreadyRunning(t *testing.T) {
	// The log raced ahead of the process lookup: nothing is left to hide.
	r, fh := holdRig(t, true)
	run := r.begin()
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n" + render + "Sound engine started\n")
	r.drive(run, time.Millisecond, "running", func() bool { return run.state.Phase == "running" })
	r.procs.add(200, 100, "javaw.exe", r.play.Add(7*time.Second))
	r.drive(run, time.Millisecond, "the game bound", func() bool { return run.bound == 200 })
	if fh.callCount() != 0 {
		t.Fatal("a running game's window is not held")
	}
}

func TestTrackerReleasesTheWindowWithoutForegroundWhenTheRunEndsOtherwise(t *testing.T) {
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
	}
	for name, end := range endings {
		t.Run(name, func(t *testing.T) {
			r, fh := holdRig(t, true)
			r.procs.add(200, 100, "javaw.exe", r.play.Add(7*time.Second))
			run := r.begin()
			r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n")
			r.drive(run, time.Millisecond, "the hold", func() bool { return fh.count() == 1 })
			end(r, run)
			if got := fh.holder(0).releases(); !reflect.DeepEqual(got, []bool{false}) {
				t.Fatalf("a window that is not being handed over is shown without the foreground: %v", got)
			}
		})
	}
}

func TestTrackerReleasesTheWindowWhenItsContextEnds(t *testing.T) {
	r, fh := holdRig(t, true)
	r.procs.add(200, 100, "javaw.exe", r.play.Add(7*time.Second))
	ctx, cancel := context.WithCancel(context.Background())
	if err := r.tracker.Track(ctx, r.request()); err != nil {
		t.Fatal(err)
	}
	until(t, "the hold", func() bool { return fh.count() == 1 })
	cancel()
	until(t, "the release", func() bool { return len(fh.holder(0).releases()) > 0 })
	if got := fh.holder(0).releases(); !reflect.DeepEqual(got, []bool{false}) {
		t.Fatalf("got %v", got)
	}
}

func TestTrackerReleasesAJavaThatWasNotTheGameAndHoldsTheNext(t *testing.T) {
	r, fh := holdRig(t, true)
	r.procs.add(150, 100, "javaw.exe", r.play.Add(time.Second))
	run := r.begin()
	r.drive(run, time.Millisecond, "packwiz held", func() bool { return fh.count() == 1 })
	r.procs.end(150, 0)
	r.drive(run, time.Millisecond, "packwiz set aside", func() bool { return run.ignored[150] })
	if got := fh.holder(0).releases(); !reflect.DeepEqual(got, []bool{false}) {
		t.Fatalf("the first hold ends with its process: %v", got)
	}
	r.procs.add(200, 100, "javaw.exe", r.play.Add(20*time.Second))
	r.log.write("first\n")
	r.drive(run, time.Millisecond, "the game held", func() bool { return fh.count() == 2 })
	if fh.holder(1).pid != 200 || len(fh.holder(1).releases()) != 0 {
		t.Fatalf("the game is held afresh: pid %d, releases %v", fh.holder(1).pid, fh.holder(1).releases())
	}
}

func TestTrackerGoesOnWhenTheWindowCannotBeHeldAndSaysSoOnce(t *testing.T) {
	logs := captureLog(t)
	r, fh := holdRig(t, true)
	fh.err = errors.New("no hook for you")
	r.procs.add(150, 100, "javaw.exe", r.play.Add(time.Second))
	run := r.begin()
	r.drive(run, time.Millisecond, "packwiz tried", func() bool { return fh.callCount() == 1 })
	r.procs.end(150, 0)
	r.drive(run, time.Millisecond, "packwiz set aside", func() bool { return run.ignored[150] })
	r.procs.add(200, 100, "javaw.exe", r.play.Add(20*time.Second))
	r.log.write("first\n")
	r.drive(run, time.Millisecond, "the game tried", func() bool { return fh.callCount() == 2 })
	r.log.append(render + "Sound engine started\n")
	r.drive(run, time.Millisecond, "running", func() bool { return run.state.Phase == "running" })
	if n := strings.Count(logs.String(), "game window cannot be held"); n != 1 {
		t.Fatalf("logged %d times:\n%s", n, logs.String())
	}
}

func TestHideTallyKeepsTheFirstAndTheWorstDelay(t *testing.T) {
	var h hideTally
	if got := h.report(); got.Hides != 0 || got.FirstHideMs != -1 || got.MaxHideMs != -1 {
		t.Fatalf("nothing hidden is reported as -1: %+v", got)
	}
	h.add(15)
	h.add(40)
	h.add(-3) // a clock that disagrees is no negative time
	got := h.report()
	if got.Hides != 3 || got.FirstHideMs != 15 || got.MaxHideMs != 40 {
		t.Fatalf("%+v", got)
	}
}

func TestTickDeltaSurvivesTheCounterWrapping(t *testing.T) {
	cases := []struct {
		now, event uint32
		want       int64
	}{
		{1000, 990, 10},
		{5, ^uint32(0) - 4, 10}, // the 32-bit tick count wrapped between the two
		{7, 7, 0},
	}
	for _, c := range cases {
		if got := tickDelta(c.now, c.event); got != c.want {
			t.Errorf("tickDelta(%d, %d) = %d, want %d", c.now, c.event, got, c.want)
		}
	}
}

func TestHoldGameWindowRefusesWhatItCannotHold(t *testing.T) {
	if runtime.GOOS == "windows" {
		if _, err := HoldGameWindow(0); err == nil {
			t.Fatal("pid 0 is no process")
		}
		return
	}
	if _, err := HoldGameWindow(1234); !errors.Is(err, errWindowHoldUnsupported) {
		t.Fatalf("got %v", err)
	}
}
