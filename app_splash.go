package main

import (
	"context"
	"log/slog"
	"time"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"

	"kapital/backend/models"
	"kapital/backend/services"
)

// splashPaintWait is how long LaunchChapter gives the webview to draw the
// loading card before the window shrinks to it: a few frames.
const splashPaintWait = 120 * time.Millisecond

// onGameState is the tracker's emit: the event, with the splash flag on it.
func (a *App) onGameState(s models.GameState) {
	s.Splash = a.splash.Observe(s.ChapterID, s.Phase)
	a.emitGameState(s)
}

func (a *App) emitGameState(s models.GameState) {
	if a.emit != nil {
		a.emit(s)
		return
	}
	if a.ctx != nil {
		wailsrt.EventsEmit(a.ctx, services.EventGameState, s)
	}
}

// appWindow is the launcher's own window for the splash (#43): the Wails
// runtime's calls on the app's context, and no window at all before startup,
// where each is a no-op that says so in the log.
type appWindow struct{ a *App }

func (w appWindow) with(call string, f func(ctx context.Context)) {
	if w.a.ctx == nil {
		slog.Debug("window call without a window", "call", call)
		return
	}
	f(w.a.ctx)
}

func (w appWindow) GetSize() (width, height int) {
	w.with("GetSize", func(ctx context.Context) { width, height = wailsrt.WindowGetSize(ctx) })
	return width, height
}

func (w appWindow) GetPosition() (x, y int) {
	w.with("GetPosition", func(ctx context.Context) { x, y = wailsrt.WindowGetPosition(ctx) })
	return x, y
}

func (w appWindow) IsMaximised() (maximised bool) {
	w.with("IsMaximised", func(ctx context.Context) { maximised = wailsrt.WindowIsMaximised(ctx) })
	return maximised
}

func (w appWindow) Maximise()   { w.with("Maximise", wailsrt.WindowMaximise) }
func (w appWindow) Unmaximise() { w.with("Unmaximise", wailsrt.WindowUnmaximise) }
func (w appWindow) Center()     { w.with("Center", wailsrt.WindowCenter) }
func (w appWindow) Minimise()   { w.with("Minimise", wailsrt.WindowMinimise) }
func (w appWindow) Unminimise() { w.with("Unminimise", wailsrt.WindowUnminimise) }

func (w appWindow) SetMinSize(width, height int) {
	w.with("SetMinSize", func(ctx context.Context) { wailsrt.WindowSetMinSize(ctx, width, height) })
}

func (w appWindow) SetSize(width, height int) {
	w.with("SetSize", func(ctx context.Context) { wailsrt.WindowSetSize(ctx, width, height) })
}

func (w appWindow) SetPosition(x, y int) {
	w.with("SetPosition", func(ctx context.Context) { wailsrt.WindowSetPosition(ctx, x, y) })
}

// leaveSplash gives the window back and tells the view the card is gone.
func (a *App) leaveSplash() {
	chapterID, left := a.splash.Leave()
	if !left {
		return
	}
	s := a.games.Latest(chapterID)
	s.Splash = false
	a.emitGameState(s)
}
