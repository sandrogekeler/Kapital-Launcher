package services

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"kapital/backend/models"
)

// Prism's console, kept (ADR-0012, amendment). On a failed start Prism
// opens its console with the error; the launcher hides it and shows its own
// view of the run, with a button to show the console, because the console holds
// what exists nowhere else (the pre-launch output: Prism never writes the launch
// log to a file). The holder therefore outlives the run. It is a per-chapter
// record: replaced at the next Play of the chapter, which closes the Prism it
// belongs to first, and released when the launcher quits. A Prism that is left
// alive on a console, shown or hidden, is the launcher's to close.
//
// The record is the launcher's note of the Prism it started for the chapter, and
// exists for every run whether or not a console was held (issue 211): with the
// splash off nothing is hidden, so the holder is nil, but a start that failed
// still leaves that Prism alive on a console of its own, and the next Play, which
// would otherwise hand its launch to it, closes it by the pid the launcher
// started. A Prism the launcher did not start has no record and is never touched.

const (
	// prismConsoleGrace is how long after a console appears, with the start
	// still waiting for the game, the run takes to end as failed. Prism logs
	// the launch step that failed a moment after it shows the console, and the
	// log's line says why (pack sync or another step), which is worth the wait.
	prismConsoleGrace = time.Second
	// prismEndWait is how long a Prism that was ended is waited for, so the next
	// one does not hand its launch to it.
	prismEndWait = 2 * time.Second
	// consoleHookWait bounds ending a hook thread (the real one waits up to
	// two seconds for it) in the time quitting allows for a Prism to close.
	consoleHookWait = 3 * time.Second
)

// prismConsole is the Prism the launcher started for a chapter's run and, when
// the run held its console, the holder that keeps it hidden.
type prismConsole struct {
	pid    int
	exited <-chan struct{}
	// holder is nil when no console was held: the splash was off, the platform
	// has no hold, or the hook could not start.
	holder ConsoleHolder
	// over is whether the run that started the Prism has ended. A Prism whose
	// run is still going has a game that is the player's and is not closed at
	// quit; one left after its run is the launcher's to close.
	over atomic.Bool

	once sync.Once
	// gone closes when the record is released, which ends its watcher.
	gone chan struct{}
}

// release ends the hook, once. What was hidden stays hidden.
func (c *prismConsole) release() {
	c.once.Do(func() {
		close(c.gone)
		if c.holder != nil {
			c.holder.Release()
		}
	})
}

// leftBehind is whether the Prism is one the launcher is to close: with a
// holder, one that still has a console window (none left means the player closed
// it, and a game may be running on that Prism); with none, one whose run has
// ended, because no console was held to say what state it is in.
func (c *prismConsole) leftBehind() bool {
	if c.holder == nil {
		return c.over.Load()
	}
	return c.holder.Held() > 0
}

// alive is whether the Prism is still running, by its own exit channel and not
// by its pid, which the OS may give to another process once it is gone.
func (c *prismConsole) alive() bool {
	select {
	case <-c.exited:
		return false
	default:
		return true
	}
}

// holdPrismConsole records the launcher's own Prism for the chapter and, with the
// dialogs' hold and on the same condition (only while the splash is on), starts
// keeping its console hidden. An earlier run's Prism left alive is closed first.
// The record is made whether or not the console can be held (issue 211). It
// never fails the run.
func (r *gameRun) holdPrismConsole() {
	if r.console != nil || !endable(r.req.Prism.PID) || r.req.Prism.Exited == nil {
		return
	}
	r.t.CloseConsole(r.req.ChapterID)
	c := &prismConsole{pid: r.req.Prism.PID, exited: r.req.Prism.Exited, gone: make(chan struct{})}
	if r.req.HoldWindow && r.t.holdConsole != nil {
		h, err := r.t.holdConsole(r.req.Prism.PID, r.consoleHeld)
		if err != nil {
			if r.t.consoleWarned.CompareAndSwap(false, true) {
				slog.Warn("prism console cannot be held", "chapter", r.req.ChapterID, "error", err)
			}
		} else {
			c.holder = h
		}
	}
	r.console = c
	r.t.mu.Lock()
	r.t.consoles[r.req.ChapterID] = c
	r.t.mu.Unlock()
	go r.t.watchConsole(r.req.ChapterID, c)
}

// consoleHeld is the holder's call when its first window is held: Prism's
// console has appeared. On Windows a run usually ends from Prism's log a moment
// before that, so the card's run report, built at the end, said the console was
// not there (#208); the request's OnConsoleHeld lets it say so now. It is called
// from the hook thread, so the request's call is made on a goroutine of its own.
func (r *gameRun) consoleHeld() {
	slog.Info("prism console held", "chapter", r.req.ChapterID)
	if r.req.OnConsoleHeld != nil {
		go r.req.OnConsoleHeld()
	}
}

// watchConsole lets the hook go when the Prism does: a Prism that has exited
// has no console left to hide, and a normal run would otherwise keep a hook
// thread until the next Play.
func (t *GameTracker) watchConsole(chapterID string, c *prismConsole) {
	select {
	case <-c.exited:
		t.mu.Lock()
		if t.consoles[chapterID] == c {
			delete(t.consoles, chapterID)
		}
		t.mu.Unlock()
		c.release()
	case <-c.gone:
	}
}

// consoleFailed is the second failure signal, Windows only: Prism's console
// appearing while the start still waits for the game's log is a launch step
// having failed, whether or not Prism's own log says which (#114 turns that
// line on). The run ends failed after a short grace for that line to be read
// (prismFailed, which is asked first, then ends it with its reason); with none
// it is "launch". A console that appears once the game's log is fresh is the
// player's own and changes nothing, and a run the player stopped keeps its reason.
func (r *gameRun) consoleFailed(now time.Time) bool {
	if r.console == nil || r.console.holder == nil || r.stop.asked || r.state.Phase != models.GamePhaseStarting || r.follower.Fresh() {
		return false
	}
	if r.console.holder.Hides() == 0 {
		return false
	}
	if r.consoleSeenAt.IsZero() {
		r.consoleSeenAt = now
		slog.Info("prism console appeared", "chapter", r.req.ChapterID)
	}
	if now.Sub(r.consoleSeenAt) < r.t.consoleGrace {
		return false
	}
	slog.Info("prism stopped the start", "chapter", r.req.ChapterID, "reason", models.GameFailLaunch, "signal", "console")
	r.state.Reason = models.GameFailLaunch
	r.set(models.GamePhaseFailed, now, nil)
	return true
}

// console is the chapter's record, nil when there is none.
func (t *GameTracker) console(chapterID string) *prismConsole {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.consoles[chapterID]
}

// consoleAvailable is whether the chapter's console can be shown: its holder has
// a window left, on a Prism that is still running. A record with no holder holds
// nothing to show.
func (t *GameTracker) consoleAvailable(chapterID string) bool {
	c := t.console(chapterID)
	return c != nil && c.holder != nil && c.alive() && c.holder.Held() > 0
}

// ShowConsole shows Prism's console for the chapter and gives it the
// foreground. False when there is none to show.
func (t *GameTracker) ShowConsole(chapterID string) (bool, error) {
	c := t.console(chapterID)
	if c == nil || c.holder == nil || !c.alive() {
		return false, nil
	}
	return c.holder.Show(), nil
}

// askPrismClose asks the Prism to close: the held console windows first, which
// are hidden and so out of the OS routine's reach, then the OS routine, which
// posts to the visible ones. A Prism that took either took it.
func (t *GameTracker) askPrismClose(chapterID string, pid int) error {
	return t.askPrismCloseVia(t.console(chapterID), pid)
}

// askPrismCloseVia is askPrismClose with the console record in hand, for a
// record already taken out of the tracker's map.
func (t *GameTracker) askPrismCloseVia(c *prismConsole, pid int) error {
	posted := 0
	if c != nil && c.holder != nil && c.pid == pid {
		posted = c.holder.Close()
	}
	err := t.os.askClose(pid)
	if err != nil && posted > 0 {
		return nil
	}
	return err
}

// CloseConsole ends the chapter's hold and, when the Prism the launcher started
// for it is still alive after its run, closes that Prism: asked as Stop does,
// ended if it is still there after stopForce. It is the next Play's first step,
// so the new Prism does not hand its launch to the old one. A chapter with no
// record is left alone, as is a Prism whose console the player closed, or whose
// run is still going: a Prism with a game running is not ours to end.
func (t *GameTracker) CloseConsole(chapterID string) {
	t.mu.Lock()
	c := t.consoles[chapterID]
	delete(t.consoles, chapterID)
	t.mu.Unlock()
	if c != nil {
		t.closeOnConsole(chapterID, c)
	}
}

func (t *GameTracker) closeOnConsole(chapterID string, c *prismConsole) {
	c.release()
	if !c.alive() || !endable(c.pid) || !c.leftBehind() {
		return
	}
	if err := t.askPrismCloseVia(c, c.pid); err != nil {
		slog.Info("prism console: no close request was taken, ending it", "chapter", chapterID, "pid", c.pid, "error", err)
	} else {
		slog.Info("prism console: close requested", "chapter", chapterID, "pid", c.pid)
		select {
		case <-c.exited:
			return
		case <-time.After(t.stopForce):
		}
	}
	if err := t.os.terminate(c.pid, true); err != nil {
		slog.Warn("prism console: end the process", "chapter", chapterID, "pid", c.pid, "error", err)
		return
	}
	slog.Info("prism console: ended", "chapter", chapterID, "pid", c.pid)
	select {
	case <-c.exited:
	case <-time.After(t.endWait):
	}
}

// Shutdown is the launcher quitting: every hook is released and each Prism left
// on a console is closed, all at once, and waited for no longer than a close
// takes at most, so quitting never hangs.
func (t *GameTracker) Shutdown() {
	t.mu.Lock()
	all := t.consoles
	t.consoles = map[string]*prismConsole{}
	t.mu.Unlock()
	var wg sync.WaitGroup
	for id, c := range all {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.closeOnConsole(id, c)
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(consoleHookWait + t.stopForce + t.endWait):
		slog.Warn("prism console: a Prism was not closed in time at quit")
	}
}
