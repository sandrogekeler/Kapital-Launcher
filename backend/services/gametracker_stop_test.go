package services

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"kapital/backend/models"
)

// wantEnded checks how a stopped run ended: the phase, the reason and that
// Play is offered again.
func (r *gameRig) wantEnded(phase string) models.GameState {
	r.t.Helper()
	r.untilPhase(phase)
	got := r.tracker.Latest("frangfurd")
	if got.Phase != phase || got.Reason != models.GameFailStopped || r.tracker.Active("frangfurd") {
		r.t.Fatalf("want %s with the reason stopped and Play offered again: %+v", phase, got)
	}
	return got
}

// A stop while the start is waiting asks the launcher's Prism to close, which
// is how Prism's console after a failed launch goes away, and the run ends
// failed when that Prism exits.
func TestStopAsksTheLaunchersPrismToCloseAndEndsWhenItExits(t *testing.T) {
	r := newGameRig(t)
	r.start()
	if err := r.tracker.Stop("frangfurd"); err != nil {
		t.Fatal(err)
	}
	if got := r.procs.closed(); !reflect.DeepEqual(got, []int{100}) || len(r.procs.terminated()) != 0 {
		t.Fatalf("Prism is asked to close and nothing is ended: closes %v, terminates %v", got, r.procs.terminated())
	}
	if got := r.tracker.Latest("frangfurd"); got.Phase != "starting" || !r.tracker.Active("frangfurd") {
		t.Fatalf("the run lasts until Prism has gone: %+v", got)
	}
	close(r.prism)
	got := r.wantEnded("failed")
	if got.ExitCode != nil {
		t.Fatalf("no game, no exit code: %+v", got)
	}
}

// A Prism that ignores the request is ended five seconds on, by the run's own
// steps, and a Prism that closes in time is never touched.
func TestStopEndsAPrismThatIgnoresTheCloseAfterFiveSeconds(t *testing.T) {
	r := newGameRig(t)
	r.start()
	if err := r.tracker.Stop("frangfurd"); err != nil {
		t.Fatal(err)
	}
	r.clock.Advance(4 * time.Second)
	time.Sleep(20 * time.Millisecond)
	if got := r.procs.terminated(); len(got) != 0 {
		t.Fatalf("not yet: %v", got)
	}
	r.clock.Advance(2 * time.Second)
	until(t, "Prism to be ended", func() bool { return len(r.procs.terminated()) > 0 })
	if got := r.procs.terminated(); !reflect.DeepEqual(got, []termCall{{100, true}}) {
		t.Fatalf("Prism, by its pid, forcibly, once: %v", got)
	}
	close(r.prism)
	r.wantEnded("failed")
	if got := r.procs.terminated(); len(got) != 1 {
		t.Fatalf("ended once: %v", got)
	}
}

func TestStopEndsAPrismThatTookNoCloseRequestAtOnce(t *testing.T) {
	r := newGameRig(t)
	r.procs.closeErr = errors.New("no window")
	r.start()
	if err := r.tracker.Stop("frangfurd"); err != nil {
		t.Fatal(err)
	}
	until(t, "Prism to be ended", func() bool { return len(r.procs.terminated()) > 0 })
	if got := r.procs.terminated(); !reflect.DeepEqual(got, []termCall{{100, true}}) {
		t.Fatalf("%v", got)
	}
	close(r.prism)
	r.wantEnded("failed")
}

// While the game runs, its Java is what is ended. Its exit takes the run
// through the usual path: crashed, with the reason.
func TestStopEndsTheGamesJavaAndTheRunEndsCrashedAsStopped(t *testing.T) {
	r := newGameRig(t)
	r.procs.add(100, 1, "prismlauncher.exe", r.play)
	r.procs.add(200, 100, "javaw.exe", r.play.Add(7*time.Second))
	r.start()
	r.log.write("[01:10:02] [main/INFO]: ModLauncher running\n" + render + "Backend library: LWJGL\n")
	r.untilPhase("window")
	if err := r.tracker.Stop("frangfurd"); err != nil {
		t.Fatal(err)
	}
	if got := r.procs.terminated(); !reflect.DeepEqual(got, []termCall{{200, false}}) || len(r.procs.closed()) != 0 {
		t.Fatalf("the Java is ended and Prism is left to quit with it: terminates %v, closes %v", got, r.procs.closed())
	}
	if !r.tracker.Active("frangfurd") {
		t.Fatal("the run lasts until the game has gone")
	}
	r.procs.end(200, 1)
	got := r.wantEnded("crashed")
	if got.ExitCode == nil || *got.ExitCode != 1 {
		t.Fatalf("the game's own exit code is kept: %+v", got)
	}
}

// SIGTERM first on macOS, and SIGKILL if the Java is still there five seconds
// later.
func TestStopForcesAGameThatIsStillThereAfterFiveSeconds(t *testing.T) {
	r := newGameRig(t)
	r.procs.add(100, 1, "prismlauncher.exe", r.play)
	r.procs.add(200, 100, "java", r.play.Add(7*time.Second))
	r.start()
	r.log.write("[01:10:02] [main/INFO]: ModLauncher running\n")
	r.untilPhase("mods")
	if err := r.tracker.Stop("frangfurd"); err != nil {
		t.Fatal(err)
	}
	r.clock.Advance(6 * time.Second)
	until(t, "the Java to be forced", func() bool { return len(r.procs.terminated()) > 1 })
	if got := r.procs.terminated(); !reflect.DeepEqual(got, []termCall{{200, false}, {200, true}}) {
		t.Fatalf("%v", got)
	}
	r.procs.end(200, 137)
	r.wantEnded("crashed")
}

// A Java the game log has not vouched for may be Prism's pre-launch one: it is
// ended, and Prism is asked to close so the start does not sit on its console.
func TestStopBeforeTheGameLogEndsTheJavaAndAsksPrismToo(t *testing.T) {
	r := newGameRig(t)
	r.procs.add(100, 1, "prismlauncher.exe", r.play)
	r.procs.add(200, 100, "javaw.exe", r.play.Add(2*time.Second))
	r.start()
	if err := r.tracker.Stop("frangfurd"); err != nil {
		t.Fatal(err)
	}
	if got := r.procs.terminated(); !reflect.DeepEqual(got, []termCall{{200, false}}) {
		t.Fatalf("%v", got)
	}
	if got := r.procs.closed(); !reflect.DeepEqual(got, []int{100}) {
		t.Fatalf("%v", got)
	}
	r.procs.end(200, 1)
	close(r.prism)
	r.wantEnded("failed")
}

// Nothing of the launcher's is alive: the launch went to a Prism that was
// already open. The run ends at once, and no process is touched.
func TestStopWithNothingAliveEndsTheRunAtOnce(t *testing.T) {
	r := newGameRig(t)
	close(r.prism)
	r.start()
	if err := r.tracker.Stop("frangfurd"); err != nil {
		t.Fatal(err)
	}
	r.wantEnded("failed")
	if len(r.procs.closed()) != 0 || len(r.procs.terminated()) != 0 {
		t.Fatalf("closes %v, terminates %v", r.procs.closed(), r.procs.terminated())
	}
}

// With no Prism pid known there is nothing to ask, and pids 0 and 1 are never
// signalled: on macOS kill(0) is the whole process group.
func TestStopNeverSignalsAPidItDoesNotOwn(t *testing.T) {
	for _, pid := range []int{0, 1} {
		r := newGameRig(t)
		req := r.request()
		req.Prism = PrismProcess{PID: pid, Exited: r.prism}
		if err := r.tracker.Track(t.Context(), req); err != nil {
			t.Fatal(err)
		}
		if err := r.tracker.Stop("frangfurd"); err != nil {
			t.Fatal(err)
		}
		r.wantEnded("failed")
		if len(r.procs.closed()) != 0 || len(r.procs.terminated()) != 0 {
			t.Fatalf("pid %d: closes %v, terminates %v", pid, r.procs.closed(), r.procs.terminated())
		}
	}
}

// A game whose log had begun and is gone, with Prism gone too, was a game: it
// is crashed, as when its process is ended, and not a start that failed.
func TestStopWithNothingAliveAfterTheGameLogBeganEndsCrashed(t *testing.T) {
	r := newGameRig(t)
	r.start()
	r.log.write("[01:10:02] [main/INFO]: ModLauncher running\n")
	r.untilPhase("mods")
	close(r.prism)
	if err := r.tracker.Stop("frangfurd"); err != nil {
		t.Fatal(err)
	}
	r.wantEnded("crashed")
}

func TestStopRefusesAChapterWithNoRunInProgress(t *testing.T) {
	r := newGameRig(t)
	if err := r.tracker.Stop("frangfurd"); !errors.Is(err, ErrNoGameToStop) {
		t.Fatalf("a chapter never launched: %v", err)
	}
	// A run that has ended is as good as none.
	r.tracker.startTimeout = time.Minute
	r.start()
	r.clock.Advance(2 * time.Minute)
	r.untilPhase("failed")
	if err := r.tracker.Stop("frangfurd"); !errors.Is(err, ErrNoGameToStop) {
		t.Fatalf("a run that has ended: %v", err)
	}
	if r.tracker.Active("frangfurd") {
		t.Fatal("Play is offered")
	}
	if len(r.procs.closed()) != 0 || len(r.procs.terminated()) != 0 {
		t.Fatal("nothing was touched")
	}
}

// A process that cannot be ended is the player's to hear about, and the run
// goes on as it was.
func TestStopReportsAGameThatCannotBeEnded(t *testing.T) {
	r := newGameRig(t)
	r.procs.add(100, 1, "prismlauncher.exe", r.play)
	r.procs.add(200, 100, "javaw.exe", r.play.Add(7*time.Second))
	r.procs.termErr = errors.New("access is denied")
	r.start()
	r.log.write("[01:10:02] [main/INFO]: ModLauncher running\n")
	r.untilPhase("mods")
	if err := r.tracker.Stop("frangfurd"); err == nil {
		t.Fatal("the failure is returned")
	}
	if got := r.tracker.Latest("frangfurd"); got.Reason != "" || !r.tracker.Active("frangfurd") {
		t.Fatalf("the run is as it was: %+v", got)
	}
	// And the player can try again.
	r.procs.mu.Lock()
	r.procs.termErr = nil
	r.procs.mu.Unlock()
	if err := r.tracker.Stop("frangfurd"); err != nil {
		t.Fatal(err)
	}
	r.procs.end(200, 1)
	r.wantEnded("crashed")
}

// A stop that arrives while the run is ending, or after, is an error and
// never a hang.
func TestStopDoesNotWaitOnARunThatIsGone(t *testing.T) {
	r := newGameRig(t)
	ctx, cancel := context.WithCancel(t.Context())
	if err := r.tracker.Track(ctx, r.request()); err != nil {
		t.Fatal(err)
	}
	cancel()
	r.tracker.runs.Wait()
	if err := r.tracker.Stop("frangfurd"); !errors.Is(err, ErrNoGameToStop) {
		t.Fatalf("%v", err)
	}
}
