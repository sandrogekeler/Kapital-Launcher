package main

import (
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

func (w launcherWindow) Minimise() {
	if w.a.ctx != nil {
		wailsrt.WindowMinimise(w.a.ctx)
	}
}

func (w launcherWindow) Unminimise() {
	if w.a.ctx != nil {
		wailsrt.WindowUnminimise(w.a.ctx)
	}
}
