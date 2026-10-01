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
	Minimise()
	Unminimise()
}

// CardActions are the two things the page can ask Go to do besides leaving,
// each the action the launcher's own window has.
type CardActions struct {
	// OpenFolder shows the chapter's instance folder.
	OpenFolder func(chapterID string) error
	// CopyLog puts the redacted tail of the log on the clipboard and returns
	// how many lines went.
	CopyLog func() (int, error)
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

	// mu guards run and the host calls made for it. Nothing that can call
	// back into the card is called with it held.
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
	errText string
	// closed is the card's window being gone because the game has the screen:
	// the launcher stays minimised until the game ends.
	closed bool
	// ended is the run having ended before the handover, with a crash or a
	// failure: the card stays, showing it, until the player leaves.
	ended bool
	// minimised is whether the launcher was minimised for this card.
	minimised bool
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
	return c.run != nil && c.run.chapter.ID == chapterID && !c.run.closed
}

// Begin opens the card for the chapter's start, centred on the launcher's
// window, and then minimises the launcher. It is called just before Prism is
// run. It returns whether the card is up: when the window cannot be opened it
// logs why and returns false, the launcher is left alone, and the caller must
// not hold the game's window for this run, because a hidden game with no card
// would show nothing.
func (c *SplashCard) Begin(chapter models.Chapter, theme string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if old := c.run; old != nil {
		// A card still up from a run that ended before its handover.
		c.run = nil
		old.host.Close()
	}
	x, y, w, h := c.cfg.Launcher.Frame()
	host := c.cfg.NewHost()
	run := &cardRun{
		chapter: splashhost.StateChapter{ID: chapter.ID, Name: chapter.Name},
		theme:   theme,
		host:    host,
		latest:  models.GameState{ChapterID: chapter.ID, Phase: models.GamePhaseStarting},
	}
	if chapter.Pack.Version != nil {
		run.chapter.PackVersion = *chapter.Pack.Version
	}
	page := c.cfg.Page
	page.OnMessage = func(msg string) { c.spawn(func() { c.handle(run, msg) }) }
	if err := host.Open(cardRect(x, y, w, h), page); err != nil {
		host.Close()
		slog.Warn("loading card cannot be shown", "chapter", chapter.ID, "error", err)
		return false
	}
	c.run = run
	c.push(run)
	c.cfg.Launcher.Minimise()
	run.minimised = true
	slog.Info("loading card opened", "chapter", chapter.ID)
	return true
}

// cardRect centres the card on the launcher's window: the card's design size
// with its centre on the window's. A window that cannot say where it is gets
// the card at the screen's corner, which still shows it.
func cardRect(x, y, w, h int) splashhost.Rect {
	cw, ch := design.SplashWidth, design.SplashHeight
	if w <= 0 || h <= 0 {
		return splashhost.Rect{W: cw, H: ch}
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
	defer c.mu.Unlock()
	run := c.run
	if run == nil || run.chapter.ID != s.ChapterID {
		return false
	}
	run.latest = s
	switch s.Phase {
	case models.GamePhaseClosed, models.GamePhaseCrashed, models.GamePhaseFailed:
		if !run.closed && s.Phase != models.GamePhaseClosed {
			run.ended = true
			c.push(run)
			return true
		}
		c.finish(run)
		return false
	case models.GamePhaseWindow, models.GamePhaseResources, models.GamePhaseRunning, models.GamePhaseStopping:
		if c.cfg.GOOS == "darwin" && !run.closed {
			c.closeCard(run)
		}
	}
	if run.closed {
		return false
	}
	c.push(run)
	return true
}

// Handover is the game having the foreground, on Windows: the card closes and
// the launcher stays minimised, to be restored when the game ends. Called by
// the tracker once its foreground release has finished, because closing first
// would hand the foreground elsewhere. It does nothing when the player left
// the card, when the run has already ended or, on macOS, ever.
func (c *SplashCard) Handover(chapterID string) {
	c.mu.Lock()
	run := c.run
	if c.cfg.GOOS != "windows" || run == nil || run.chapter.ID != chapterID || run.closed || run.ended {
		c.mu.Unlock()
		return
	}
	c.closeCard(run)
	c.mu.Unlock()
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
	if run == nil || run.closed || (only != nil && run != only) {
		c.mu.Unlock()
		return "", false
	}
	c.finish(run)
	c.mu.Unlock()
	slog.Info("loading card left", "chapter", run.chapter.ID)
	c.changed(run.chapter.ID)
	return run.chapter.ID, true
}

// Shutdown closes a card that is still up when the app quits.
func (c *SplashCard) Shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.run != nil {
		c.run.host.Close()
		c.run = nil
	}
}

// closeCard closes the window because the game has the screen. mu is held.
func (c *SplashCard) closeCard(run *cardRun) {
	run.closed = true
	run.host.Close()
	slog.Info("loading card handed over", "chapter", run.chapter.ID)
}

// finish ends the run: the window closes if it is still up and the launcher is
// restored. mu is held.
func (c *SplashCard) finish(run *cardRun) {
	c.run = nil
	if !run.closed {
		run.host.Close()
	}
	if run.minimised {
		c.cfg.Launcher.Unminimise()
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
	return c.run == run && !run.closed
}

// handle is a message from the page: leave, or one of the two actions, whose
// outcome goes back into the card's state. Anything the page says that is not
// one of the three is dropped by ParseMessage. The chapter is the run's own.
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
	if c.run != run || run.closed {
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
