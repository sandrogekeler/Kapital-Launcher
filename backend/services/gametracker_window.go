package services

import (
	"log/slog"

	"kapital/backend/models"
)

// holdWindow starts keeping the game's window hidden, when asked to and while
// the handover has not come. It never fails the run: where it cannot hold, the
// window shows as the game makes it.
func (r *gameRun) holdWindow(pid int) {
	if !r.req.HoldWindow || r.t.hold == nil || r.holder != nil ||
		phaseRank[r.state.Phase] >= phaseRank[models.GamePhaseResources] {
		return
	}
	h, err := r.t.hold(pid)
	if err != nil {
		if r.t.holdWarned.CompareAndSwap(false, true) {
			slog.Warn("game window cannot be held", "chapter", r.req.ChapterID, "error", err)
		}
		return
	}
	r.holder = h
}

// handover is the game having the screen: the held window, if there is one,
// is shown and given the foreground, and then OnHandover is called, once per
// run. The foreground release nudges the window and so takes a few hundred ms:
// it runs on its own goroutine with the callback after it, and the run loop
// goes on. With no window held there is nothing to wait for and the callback
// runs here.
func (r *gameRun) handover() {
	if r.handedOver {
		return
	}
	r.handedOver = true
	// The game has the screen: Prism's progress dialogs are over, so the hook
	// ends and nothing is shown (Prism closes them itself).
	r.releaseDialogs(false)
	h := r.holder
	r.holder = nil
	done := r.req.OnHandover
	if h == nil {
		if done != nil {
			done()
		}
		return
	}
	go func() {
		r.logRelease(true, h.Release(true))
		if done != nil {
			done()
		}
	}()
}

// releaseWindow shows the held window again without the foreground and logs
// what holding it came to. It is quick and done before the run moves on, so a
// window is never left hidden behind a run that ended.
func (r *gameRun) releaseWindow(foreground bool) {
	if r.holder == nil {
		return
	}
	h := r.holder
	r.holder = nil
	r.logRelease(foreground, h.Release(foreground))
}

// holdPrismDialogs starts hiding the launcher's own Prism's "Please wait"
// dialogs (#95), when asked to: the same HoldWindow that holds the game's
// window, so only while the splash is on. It never fails the run.
//
// [verify] Play while Prism was already open: the launcher's Prism then hands
// the launch to that one and exits at once, and the dialogs are the other
// Prism's, which this does not hold (the hook is on the launcher's own pid;
// step ends it when that Prism exits). Not yet seen on a real install (#44).
func (r *gameRun) holdPrismDialogs() {
	if !r.req.HoldWindow || r.t.holdDialogs == nil || r.dialogs != nil || r.req.Prism.PID <= 0 {
		return
	}
	d, err := r.t.holdDialogs(r.req.Prism.PID)
	if err != nil {
		if r.t.dialogsWarned.CompareAndSwap(false, true) {
			slog.Warn("prism dialogs cannot be held", "chapter", r.req.ChapterID, "error", err)
		}
		return
	}
	r.dialogs = d
}

// releaseDialogs ends the hold on Prism's dialogs and logs what it came to.
// With show, a dialog that still exists is shown again: a run that ends before
// the handover must not leave an error Prism is showing hidden.
func (r *gameRun) releaseDialogs(show bool) {
	if r.dialogs == nil {
		return
	}
	d := r.dialogs
	r.dialogs = nil
	rep := d.Release(show)
	slog.Info("prism dialogs", "chapter", r.req.ChapterID, "hides", rep.Hides, "shownBack", rep.ShownBack)
}

// logRelease records what holding a window came to: counts and milliseconds,
// never a window title.
func (r *gameRun) logRelease(handover bool, rep WindowReport) {
	slog.Info("game window", "chapter", r.req.ChapterID, "handover", handover,
		"seen", rep.Seen, "swept", rep.Swept, "hides", rep.Hides,
		"firstHideMs", rep.FirstHideMs, "maxHideMs", rep.MaxHideMs,
		"foreground", rep.Foreground, "nudged", rep.Nudged)
}
