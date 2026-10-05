package services

import "log/slog"

// errConsoleGone is what the card says when the console it offered is no longer
// there: the player closed it, or Prism has gone.
const errConsoleGone = "Prism's console is no longer open"

// ConsoleHeld is the tracker's call when the hold first catches Prism's console
// (TrackRequest.OnConsoleHeld). On Windows a failed start usually ends from
// Prism's log about 0.4 s before Prism opens its console, so the report the
// card built at the end said there was none (#208). The card's report is
// changed now and the card pushed again, but only for a card that is up, shows
// an end and has a report (a run the player stopped has none); the report built
// after the console came already knows. Event driven: nothing waits or polls.
func (c *SplashCard) ConsoleHeld(chapterID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	run := c.run
	if run == nil || run.opening || run.chapter.ID != chapterID {
		return
	}
	run.consoleHeld = true
	if !c.live(run) || !run.ended || run.report == nil || run.report.ConsoleAvailable {
		return
	}
	run.report.ConsoleAvailable = true
	c.push(run)
}

// showConsole is the page's showConsole action: Prism's console, which the
// launcher hid when the start failed, is shown and given the foreground. The
// chapter is the run's own, and the outcome goes into the card like openFolder's:
// nothing on success, a line on a failure.
func (c *SplashCard) showConsole(run *cardRun) {
	if c.cfg.Actions.ShowConsole == nil {
		return
	}
	shown, err := c.cfg.Actions.ShowConsole(run.chapter.ID)
	if err != nil {
		slog.Warn("show prism console", "chapter", run.chapter.ID, "error", err)
	}
	c.report(run, func() {
		run.errText = ""
		switch {
		case err != nil:
			run.errText = err.Error()
		case !shown:
			run.errText = errConsoleGone
		}
	})
}
