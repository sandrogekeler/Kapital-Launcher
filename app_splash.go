package main

import (
	"context"
	"io/fs"
	"path/filepath"
	"runtime"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"

	"kapital/backend/design"
	"kapital/backend/models"
	"kapital/backend/services"
	"kapital/backend/splashhost"
)

// splashEntry is the card's page in the embedded build (frontend/splash.html).
const splashEntry = "splash.html"

// newSplashCard wires the loading card (#97): a window of its own over the
// embedded build in dist, the launcher's window to centre on and step aside
// from, and the two actions the page may ask for.
func (a *App) newSplashCard(dist fs.FS, newHost func() splashhost.Host) *services.SplashCard {
	return services.NewSplashCard(services.CardConfig{
		GOOS:     runtime.GOOS,
		Launcher: launcherWindow{a},
		NewHost:  newHost,
		Page: splashhost.Page{
			Assets:  splashhost.AssetsFrom(dist),
			Entry:   splashEntry,
			DataDir: filepath.Join(a.dataDir, "splash-webview"),
			// The dark ground: the card is mostly artwork, drawn over it.
			Background: design.WindowBackground,
		},
		Actions: services.CardActions{OpenFolder: a.OpenInstanceFolder, CopyLog: a.CopyRedactedLog},
		// The tracker is made after the card, so it is looked up when needed.
		Report:  func(id string) (models.RunReport, error) { return a.games.Report(id) },
		Changed: a.splashChanged,
	})
}

// beginSplash opens the card for a start when the player's setting says so and
// there is a window to centre it on, and reports whether it is up.
func (a *App) beginSplash(chapter models.Chapter, settings models.AppSettings) bool {
	if a.ctx == nil || !services.LoadingSplashOn(runtime.GOOS, settings) {
		return false
	}
	return a.splash.Begin(chapter, services.CardTheme(settings.Theme))
}

// onGameState is the tracker's emit: the event, with the splash flag on it.
func (a *App) onGameState(s models.GameState) {
	s.Splash = a.splash.Observe(s)
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

// splashChanged tells the view a chapter's card has gone without a game event:
// its latest state, with the splash flag as it is now.
func (a *App) splashChanged(chapterID string) {
	s := a.games.Latest(chapterID)
	s.Splash = a.splash.Showing(chapterID)
	a.emitGameState(s)
}

// windowCalls are the Wails runtime's window calls launcherWindow makes. They
// are a field of App so a test can stand in for the window, which it cannot
// have: the runtime stops the process on a context that is not Wails'.
type windowCalls struct {
	minimise   func(context.Context)
	unminimise func(context.Context)
	show       func(context.Context)
	// isFullscreen is whether the window is in full screen.
	isFullscreen func(context.Context) bool
	// showAfterUnminimise is whether bringing the window back also needs show.
	showAfterUnminimise bool
}

// wailsWindowCalls is the real window. macOS needs show after unminimise:
// Wails' WindowUnminimise is deminiaturize: alone, which only de-minimises, and
// the app is inactive by then because the game was frontmost, so the launcher
// would come back behind other windows. WindowShow is makeKeyAndOrderFront
// plus activateIgnoringOtherApps. Windows does not need it: PR #102 saw the
// launcher come back in front there.
func wailsWindowCalls() windowCalls {
	return windowCalls{
		minimise:            wailsrt.WindowMinimise,
		unminimise:          wailsrt.WindowUnminimise,
		show:                wailsrt.WindowShow,
		isFullscreen:        wailsrt.WindowIsFullscreen,
		showAfterUnminimise: runtime.GOOS == "darwin",
	}
}

// launcherWindow is the launcher's own window for the card: the Wails
// runtime's calls on the app's context, and no window at all before startup,
// where each is a no-op that says nothing.
type launcherWindow struct{ a *App }

func (w launcherWindow) Frame() (x, y, width, height int) {
	if w.a.ctx == nil {
		return 0, 0, 0, 0
	}
	x, y = wailsrt.WindowGetPosition(w.a.ctx)
	width, height = wailsrt.WindowGetSize(w.a.ctx)
	return x, y, width, height
}

// Minimise and Unminimise leave a full screen launcher alone, on every OS. On
// macOS miniaturize: does nothing to a full screen window, and the card, which
// opens over the launcher's Space (host_darwin.m), is what the player sees; a
// full screen launcher simply stays under it and is still there when the game
// ends.
func (w launcherWindow) Minimise() {
	if w.a.ctx == nil || w.a.window.isFullscreen(w.a.ctx) {
		return
	}
	w.a.window.minimise(w.a.ctx)
}

func (w launcherWindow) Unminimise() {
	if w.a.ctx == nil || w.a.window.isFullscreen(w.a.ctx) {
		return
	}
	w.a.window.unminimise(w.a.ctx)
	if w.a.window.showAfterUnminimise {
		w.a.window.show(w.a.ctx)
	}
}
