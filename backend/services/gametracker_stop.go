package services

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"kapital/backend/models"
)

// Stopping a run from the launcher (the Stop button). A start that stalls, or
// a game that has died with Prism still up, would otherwise hold Play until
// the tracker gave up. Stop is a request to the run's own goroutine, so every
// process call stays where the others are, and it ends only what the tracker
// found itself: the Prism the launcher started (asked to close, then ended)
// and the game's Java found as that Prism's child (ended), by pid and never by
// name (S3.9).

// ErrNoGameToStop is Stop on a chapter with no run in progress.
var ErrNoGameToStop = errors.New("there is no game to stop")

// gameStopForce is how long a process asked to end has before it is made to.
const gameStopForce = 5 * time.Second

// stopRequest is one Stop, answered once the run has acted on it.
type stopRequest struct{ reply chan error }

// runStop is what a run keeps for Stop. requests and done are made with the
// run and never change; the rest is the run's goroutine's alone.
type runStop struct {
	requests chan stopRequest
	// done closes when the run's goroutine ends, so a Stop that arrives late
	// does not wait for a loop that is gone.
	done chan struct{}
	// asked is whether a stop has been requested. The run then ends as soon as
	// what it asked to end has, without the grace a Prism that went on its own
	// is given.
	asked bool
	// forceAt is when a process that was asked to end and is still there is
	// made to; zero when nothing is pending. Judged on the next steps, not
	// slept on.
	forceAt time.Time
}

func newRunStop() runStop {
	return runStop{requests: make(chan stopRequest), done: make(chan struct{})}
}

// register makes the run reachable by Stop, from the moment its goroutine
// starts.
func (t *GameTracker) register(r *gameRun) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.live == nil {
		t.live = map[string]*gameRun{}
	}
	t.live[r.req.ChapterID] = r
}

// finish is the run's goroutine ending: Stop can no longer reach it.
func (r *gameRun) finish() {
	r.t.mu.Lock()
	if r.t.live[r.req.ChapterID] == r {
		delete(r.t.live, r.req.ChapterID)
	}
	r.t.mu.Unlock()
	close(r.stop.done)
}

// Stop ends a chapter's run now: the game's Java is ended, or, before there is
// one, the Prism the launcher started is asked to close and ended if it does
// not. The run then finishes through its usual path, as crashed or failed with
// the reason "stopped", and Play is offered again. A chapter with no run in
// progress is an error.
func (t *GameTracker) Stop(chapterID string) error {
	t.mu.Lock()
	r := t.live[chapterID]
	t.mu.Unlock()
	if r == nil {
		return ErrNoGameToStop
	}
	req := stopRequest{reply: make(chan error, 1)}
	select {
	case r.stop.requests <- req:
		// The loop answers every request it takes, and done closes only after.
		return <-req.reply
	case <-r.stop.done:
		return ErrNoGameToStop
	}
}

// serveStop answers a Stop on the run's goroutine, and reports whether the run
// has ended by it.
func (r *gameRun) serveStop(req stopRequest) bool {
	if !GamePhaseActive(r.state.Phase) {
		req.reply <- ErrNoGameToStop
		return false
	}
	ended, err := r.stopNow(r.t.now())
	req.reply <- err
	return ended
}

// stopNow ends what there is to end, in order: the game's Java, else the
// launcher's Prism, else (nothing of ours is alive, the launch went to another
// Prism) the run itself.
func (r *gameRun) stopNow(now time.Time) (ended bool, err error) {
	// A Java that came up since the last look is the one to end, not Prism.
	if r.bound == 0 && !r.procUnavailable {
		r.lastProcPoll = now
		r.findGame()
	}
	switch {
	case r.bound != 0:
		return false, r.stopGame(now)
	case r.prismAlive():
		r.stopPrism(now)
		return false, nil
	}
	r.markStopped()
	// A game log that had begun means a game did, and it is gone: crashed, as
	// when its process is ended. Without one, no game ever appeared.
	phase := models.GamePhaseFailed
	if r.follower.Fresh() {
		phase = models.GamePhaseCrashed
	}
	slog.Info("stop", "chapter", r.req.ChapterID, "target", "none")
	r.set(phase, now, nil)
	return true, nil
}

// stopGame ends the bound Java. Its exit is seen by the wait already running,
// and gameExited ends the run.
func (r *gameRun) stopGame(now time.Time) error {
	if !endable(r.bound) {
		return fmt.Errorf("the game's process %d cannot be ended", r.bound)
	}
	if err := r.t.os.terminate(r.bound, false); err != nil {
		return fmt.Errorf("end the game: %w", err)
	}
	slog.Info("stop", "chapter", r.req.ChapterID, "target", "game", "pid", r.bound)
	r.markStopped()
	r.stop.forceAt = now.Add(r.t.stopForce)
	if !r.follower.Fresh() && r.prismAlive() {
		// A Java the log has not vouched for may be Prism's pre-launch one,
		// whose end Prism reports on its console and then waits there: Prism
		// is asked to close as well, so the start does not sit on it.
		r.stopPrism(now)
	}
	return nil
}

// stopPrism asks the launcher's Prism to close, and arms ending it if it has
// not by stopForce. A Prism that took no request (no window of its own yet) is
// ended at the next step.
func (r *gameRun) stopPrism(now time.Time) {
	pid := r.req.Prism.PID
	r.markStopped()
	r.stop.forceAt = now.Add(r.t.stopForce)
	if err := r.t.os.askClose(pid); err != nil {
		slog.Info("stop: prism took no close request, ending it", "chapter", r.req.ChapterID, "pid", pid, "error", err)
		r.stop.forceAt = now
		return
	}
	slog.Info("stop", "chapter", r.req.ChapterID, "target", "prism", "pid", pid)
}

// markStopped records that the player ended the run, which is the reason the
// end of it carries.
func (r *gameRun) markStopped() {
	r.stop.asked = true
	r.state.Reason = models.GameFailStopped
}

// escalateStop makes the process that was asked to end, and is still there,
// end. Judged on the run's steps, so a Prism that closes by itself in time is
// never touched.
func (r *gameRun) escalateStop(now time.Time) {
	if r.stop.forceAt.IsZero() || now.Before(r.stop.forceAt) {
		return
	}
	r.stop.forceAt = time.Time{}
	pid, target := r.bound, "game"
	if pid == 0 && r.prismAlive() {
		pid, target = r.req.Prism.PID, "prism"
	}
	if !endable(pid) {
		return
	}
	if err := r.t.os.terminate(pid, true); err != nil {
		slog.Warn("stop: end the process", "chapter", r.req.ChapterID, "target", target, "pid", pid, "error", err)
		return
	}
	slog.Info("stop: ended", "chapter", r.req.ChapterID, "target", target, "pid", pid)
}

// prismAlive is whether the Prism the launcher started is still running, by
// its own exit channel and not by its pid, which the OS may hand to another
// process once it is gone.
func (r *gameRun) prismAlive() bool {
	if !endable(r.req.Prism.PID) || !r.prismExitedAt.IsZero() {
		return false
	}
	select {
	case <-r.req.Prism.Exited:
		return false
	default:
		return true
	}
}

// prismWait is how long after the launcher's Prism has exited, with no game
// found, a start counts as failed: the grace for a Prism that went on its own,
// none for one the player stopped.
func (r *gameRun) prismWait() time.Duration {
	if r.stop.asked {
		return 0
	}
	return r.t.prismGrace
}

// endable is whether a pid is one a process call may be made on. Pid 0 and 1
// are never ours, and on macOS kill takes 0 for the caller's whole process
// group.
func endable(pid int) bool {
	return pid > 1
}
