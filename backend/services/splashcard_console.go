package services

import "log/slog"

// errConsoleGone is what the card says when the console it offered is no longer
// there: the player closed it, or Prism has gone.
const errConsoleGone = "Prism's console is no longer open"

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
