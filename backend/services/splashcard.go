package services

import (
	"log/slog"
	"sync"

	"kapital/backend/design"
	"kapital/backend/models"
	"kapital/backend/splashhost"
)

// CardLauncher is what the card asks of the launcher's own window (#97): where
// it is, so the card opens centred on it, and to step aside and come back. The
// app implements it over the Wails runtime; every call is Go's, the frontend
// never moves the window.
type CardLauncher interface {
	// Frame is the window's position and size in screen coordinates, as the
	// Wails runtime reports them; zero sizes when it cannot say.
	Frame() (x, y, w, h int)
	// Minimised is whether the window is minimised, when Frame is no place to
	// centre on: Windows parks a minimised window at about (-32000, -32000).
	Minimised() bool
	Minimise()
	Unminimise()
}

// CardActions are the three things the page can ask Go to do besides leaving,
// each the action the launcher's own window has.
type CardActions struct {
	// OpenFolder shows the chapter's instance folder.
	OpenFolder func(chapterID string) error
	// CopyLog puts the redacted tail of the log on the clipboard and returns
	// how many lines went.
	CopyLog func() (int, error)
	// ShowConsole shows the console of the chapter's Prism, which the launcher
	// hid, and says whether there was one to show (splashcard_console.go).
	ShowConsole func(chapterID string) (bool, error)
}

// CardConfig is what a SplashCard is made from.
type CardConfig struct {
	// GOOS decides where the handover is: Windows closes the card when the
	// game's window is released (Handover), macOS at the game's "window"
	// phase, where there is no hold (Observe).
	GOOS     string
	Launcher CardLauncher
	// NewHost makes the window of one run; splashhost.New in the app.
	NewHost func() splashhost.Host
	// Page is what each window shows; its OnMessage is the card's own.
	Page    splashhost.Page
	Actions CardActions
	// Report builds the run report the card shows when a run ends crashed or
	// failed (ADR-2, sixth amendment); the tracker's Report in the app. Nil
	// shows the card without one.
	Report func(chapterID string) (models.RunReport, error)
	// Changed is called, with no lock held, when the card goes away without a
	// game event to say so (the handover, the player leaving), so the view can
	// be told the splash flag is false.
	Changed func(chapterID string)
}

// SplashCard is the loading card for a start (#97): a window of its own,
// opened before Prism is run, kept up to date with the game's phases and
// closed at the handover. The launcher's window minimises while the card is up
// and is restored when the game ends, showing how it ended. It makes every
// window call, on the tracker's phase changes, so a reload of the view cannot
// leave anything in the wrong shape. One card at a time: there is one game.
type SplashCard struct {
	cfg CardConfig
	// spawn runs a page message's action; a goroutine, so the host's UI thread
	// is never held by it. A test runs it inline.
	spawn func(func())

	// mu guards run. It is never held across a call that waits on the host's
	// UI thread (Open, Close): at quit Shutdown takes it on the main thread
	// after the main loop has returned, where nothing answers such a wait.
	// Nothing that can call back into the card is called with it held.
	mu  sync.Mutex
	run *cardRun
}

// cardRun is one start's card.
type cardRun struct {
	chapter splashhost.StateChapter
	theme   string
	host    splashhost.Host
	latest  models.GameState
	copyLog *splashhost.StateCopyLog
	// report is the run's report, filled once when it ends badly.
	report *models.RunReport
	// consoleHeld is Prism's console having been caught by the hold during this
	// run (ConsoleHeld), whatever the run had reached by then: the report built
	// at the end is told of it, so a console that came first is not missed.
	consoleHeld bool
	errText     string
	// closed is the card's window being gone because the game has the screen:
	// the launcher stays minimised until the game ends.
	closed bool
	// ended is the run having ended before the handover, with a crash or a
	// failure: the card stays, showing it, until the player leaves.
	ended bool
	// minimised is whether the launcher was minimised for this card.
	minimised bool
	// opening is Begin being inside host.Open, with mu released. The run is
	// in c.run so a second Begin is refused and Shutdown can find the host,
	// but it is no card yet: everything but those two leaves it alone.
	opening bool
}

// NewSplashCard makes the card for cfg.
func NewSplashCard(cfg CardConfig) *SplashCard {
	return &SplashCard{cfg: cfg, spawn: func(f func()) { go f() }}
}

// HoldsGameWindow is whether this OS's start keeps the game's window hidden
// until the handover (ADR-0012): Windows only. macOS has no such hold, so its
// card closes when the game's window appears instead.
func (c *SplashCard) HoldsGameWindow() bool { return c.cfg.GOOS == "windows" }

// Showing is whether the chapter's run has its card up.
func (c *SplashCard) Showing(chapterID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.run != nil && c.run.chapter.ID == chapterID && c.live(c.run)
}

// live is whether run is the card up now, opened and not closed. mu is held.
func (c *SplashCard) live(run *cardRun) bool {
	return c.run == run && !run.opening && !run.closed
}

// Begin opens the card for the chapter's start, centred on the launcher's
// window, and then minimises the launcher. It is called just before Prism is
// run. It returns whether the card is up: when the window cannot be opened it
// logs why and returns false, the launcher is left alone, and the caller must
// not hold the game's window for this run, because a hidden game with no card
// would show nothing. It waits for the page without holding the card's lock,
// so a quit meanwhile (Shutdown) ends the wait, and Begin then returns false.
func (c *SplashCard) Begin(chapter models.Chapter, theme string) bool {
	x, y, w, h := c.cfg.Launcher.Frame()
	rect := cardRect(x, y, w, h, c.cfg.Launcher.Minimised())
	host := c.cfg.NewHost()
	run := &cardRun{
		chapter: splashhost.StateChapter{ID: chapter.ID, Name: chapter.Name},
		theme:   theme,
		host:    host,
		latest:  models.GameState{ChapterID: chapter.ID, Phase: models.GamePhaseStarting},
		opening: true,
	}
	if chapter.Pack.Version != nil {
		run.chapter.PackVersion = *chapter.Pack.Version
	}
	page := c.cfg.Page
	page.OnMessage = func(msg string) { c.spawn(func() { c.handle(run, msg) }) }

	c.mu.Lock()
	old := c.run
	if old != nil && old.opening {
		c.mu.Unlock()
		slog.Warn("loading card is already opening", "chapter", chapter.ID)
		return false
	}
	c.run = run
	var stale splashhost.Host
	if old != nil && !old.closed {
		// A card still up from a run that ended before its handover.
		old.closed = true
		stale = old.host
	}
	c.mu.Unlock()
	if stale != nil {
		stale.Close()
	}

	// Open waits for the page, up to 15 s, so mu is not held: Shutdown must be
	// able to reach this host meanwhile, and its Close ends the wait.
	err := host.Open(rect, page)

	c.mu.Lock()
	if err != nil || c.run != run {
		if c.run == run {
			c.run = nil
		}
		c.mu.Unlock()
		// A card shut down while it opened is closed again here: the window
		// may have come up after Shutdown's Close. Close is safe twice.
		host.Close()
		if err != nil {
			slog.Warn("loading card cannot be shown", "chapter", chapter.ID, "error", err)
		} else {
			slog.Info("loading card closed while opening", "chapter", chapter.ID)
		}
		return false
	}
	run.opening = false
	c.push(run)
	c.cfg.Launcher.Minimise()
	run.minimised = true
	c.mu.Unlock()
	slog.Info("loading card opened", "chapter", chapter.ID)
	return true
}

// cardRect centres the card on the launcher's window: the card's design size
// with its centre on the window's. A window that is minimised, or cannot say
// where it is, has no place worth centring on (a minimised one is parked at
// about (-32000, -32000), where the card was once opened out of reach, #210),
// so the rect asks the host for the screen's middle instead (OnScreen). A
// frame that is on no monitor for another reason is the host's to catch, as it
// alone knows the monitors (splashhost.placeOnScreen).
func cardRect(x, y, w, h int, minimised bool) splashhost.Rect {
	cw, ch := design.SplashWidth, design.SplashHeight
	if minimised || w <= 0 || h <= 0 {
		return splashhost.Rect{W: cw, H: ch, OnScreen: true}
	}
	return splashhost.Rect{X: x + (w-cw)/2, Y: y + (h-ch)/2, W: cw, H: ch}
}

// Observe is called for each game state of a chapter's run, and returns
// whether that event shows the card. The card is updated; on macOS the card
// closes at the game's window. A run that ends gives the launcher back, except
// one that crashed or failed before the handover: the card stays, to say so,
// and the player leaves it.
func (c *SplashCard) Observe(s models.GameState) bool {
	c.mu.Lock()
	run := c.run
	if run == nil || run.opening || run.chapter.ID != s.ChapterID {
		c.mu.Unlock()
		return false
	}
	run.latest = s
	var after func()
	switch s.Phase {
	case models.GamePhaseClosed, models.GamePhaseCrashed, models.GamePhaseFailed:
		if !run.closed && s.Phase != models.GamePhaseClosed {
			run.ended = true
			c.mu.Unlock()
			c.showEnd(run, s)
			return true
		}
		after = c.finish(run)
		c.mu.Unlock()
		after()
		return false
	case models.GamePhaseWindow, models.GamePhaseResources, models.GamePhaseRunning, models.GamePhaseStopping:
		if c.cfg.GOOS == "darwin" && !run.closed {
			after = c.closeCard(run)
		}
	}
	shows := !run.closed
	if shows {
		c.push(run)
	}
	c.mu.Unlock()
	if after != nil {
		after()
	}
	return shows
}

// showEnd pushes the card's state for a run that ended before the handover,
// with the run's report. The report reads the disk, so it is built with mu
// released, as the host's own calls are, and the card is pushed once, with it,
// rather than twice. A card the player left meanwhile is not pushed to.
func (c *SplashCard) showEnd(run *cardRun, s models.GameState) {
	var report *models.RunReport
	// A run the player stopped from the launcher is not one that went wrong: it
	// has nothing to explain.
	if c.cfg.Report != nil && s.Reason != models.GameFailStopped {
		r, err := c.cfg.Report(run.chapter.ID)
		if err != nil {
			slog.Warn("run report for the card", "chapter", run.chapter.ID, "error", err)
		} else {
			report = &r
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.live(run) {
		return
	}
	// The hold may have caught the console while the report was being read: its
	// call (ConsoleHeld) then found no report to change, and it is told here.
	if report != nil && run.consoleHeld {
		report.ConsoleAvailable = true
	}
	run.report = report
	c.push(run)
}

// Handover is the game having the foreground, on Windows: the card closes and
// the launcher stays minimised, to be restored when the game ends. Called by
// the tracker once its foreground release has finished, because closing first
// would hand the foreground elsewhere. It does nothing when the player left
// the card, when the run has already ended or, on macOS, ever.
func (c *SplashCard) Handover(chapterID string) {
	c.mu.Lock()
	run := c.run
	if c.cfg.GOOS != "windows" || run == nil || run.opening || run.chapter.ID != chapterID || run.closed || run.ended {
		c.mu.Unlock()
		return
	}
	after := c.closeCard(run)
	c.mu.Unlock()
	after()
	c.changed(chapterID)
}

// Leave closes the card at once and brings the launcher back, whatever the
// run has reached, and says which chapter's card it was. No card up is not an
// error: it returns false. The run goes on, and the game still appears at its
// handover; the launcher just does nothing more then. Changed is called, so
// the view hears the card is gone.
func (c *SplashCard) Leave() (string, bool) { return c.leave(nil) }

// leave is Leave for the card up now, or, with a run, for that run's card only.
func (c *SplashCard) leave(only *cardRun) (string, bool) {
	c.mu.Lock()
	run := c.run
	if run == nil || run.opening || run.closed || (only != nil && run != only) {
		c.mu.Unlock()
		return "", false
	}
	after := c.finish(run)
	c.mu.Unlock()
	after()
	slog.Info("loading card left", "chapter", run.chapter.ID)
	c.changed(run.chapter.ID)
	return run.chapter.ID, true
}

// Shutdown closes a card that is still up when the app quits, or one that is
// still opening: Begin is then inside host.Open, and the host's Close is what
// ends that wait, so it is called here and not left to Begin.
func (c *SplashCard) Shutdown() {
	c.mu.Lock()
	run := c.run
	c.run = nil
	var host splashhost.Host
	if run != nil && !run.closed {
		run.closed = true
		host = run.host
	}
	c.mu.Unlock()
	if host != nil {
		host.Close()
	}
}

// closeCard marks the window as closing because the game has the screen, and
// returns the closing itself for the caller to run once mu is released. mu is
// held.
func (c *SplashCard) closeCard(run *cardRun) func() {
	run.closed = true
	slog.Info("loading card handed over", "chapter", run.chapter.ID)
	return run.host.Close
}

// finish ends the run: it is taken out at once, and the returned call, to be
// made once mu is released, closes the window if it is still up and restores
// the launcher. mu is held.
func (c *SplashCard) finish(run *cardRun) func() {
	c.run = nil
	host := run.host
	if run.closed {
		host = nil
	}
	run.closed = true
	minimised := run.minimised
	return func() {
		if host != nil {
			host.Close()
		}
		if minimised {
			c.cfg.Launcher.Unminimise()
		}
	}
}

func (c *SplashCard) changed(chapterID string) {
	if c.cfg.Changed != nil {
		c.cfg.Changed(chapterID)
	}
}

// push sends the card its state. mu is held.
func (c *SplashCard) push(run *cardRun) {
	game := run.latest
	game.Splash = true
	state := splashhost.State{
		Chapter: run.chapter,
		Game:    game,
		CopyLog: run.copyLog,
		Report:  run.report,
		Error:   run.errText,
		Theme:   run.theme,
	}
	body, err := splashhost.StateJSON(state)
	if err != nil {
		slog.Error("loading card state", "error", err)
		return
	}
	run.host.Update(body)
}

// up is whether run's card is the one up now: a message from a card that has
// gone since is a stale one and is dropped.
func (c *SplashCard) up(run *cardRun) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.live(run)
}

// handle is a message from the page: leave, or one of the three actions, whose
// outcome goes back into the card's state. Anything the page says that is not
// one of the four is dropped by ParseMessage. The chapter is the run's own.
func (c *SplashCard) handle(run *cardRun, msg string) {
	action, ok := splashhost.ParseMessage(msg)
	if !ok || !c.up(run) {
		return
	}
	switch action {
	case splashhost.ActionLeave:
		c.leave(run)
	case splashhost.ActionOpenFolder:
		err := c.cfg.Actions.OpenFolder(run.chapter.ID)
		c.report(run, func() {
			run.copyLog, run.errText = nil, ""
			if err != nil {
				run.errText = err.Error()
			}
		})
	case splashhost.ActionShowConsole:
		c.showConsole(run)
	case splashhost.ActionCopyLog:
		lines, err := c.cfg.Actions.CopyLog()
		c.report(run, func() {
			run.errText = ""
			run.copyLog = &splashhost.StateCopyLog{Lines: &lines}
			if err != nil {
				run.copyLog = &splashhost.StateCopyLog{Error: err.Error()}
			}
		})
	}
}

// report changes the card's state after an action and pushes it, unless the
// card has gone meanwhile.
func (c *SplashCard) report(run *cardRun, change func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.live(run) {
		return
	}
	change()
	c.push(run)
}

// CardTheme is the theme setting as the page takes it: "dark" or "light", and
// "" for the system's own, which the page then follows.
func CardTheme(theme string) string {
	switch theme {
	case "dark", "light":
		return theme
	}
	return ""
}
