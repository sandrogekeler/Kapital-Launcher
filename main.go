package main

import (
	"embed"
	"io/fs"
	"log/slog"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"kapital/backend/design"
	"kapital/backend/services"
)

// The built frontend. `pnpm build` in frontend/ has to run before `go build`
// or `wails build`; frontend/dist/.gitkeep keeps the directory present so a
// bare `go vet ./...` compiles on a fresh clone.
//
//go:embed all:frontend/dist
var assets embed.FS

// The launcher manifest built into this version of the app. The Kapitel
// Kapital site is planned to publish the same shape, at which point this copy
// becomes the offline fallback (docs/adr/0004-launcher-manifest.md).
//
//go:embed data/launcher.json
var bundledManifest []byte

func main() {
	// Before wails.Run, so a failure to open the window is itself logged
	// somewhere retrievable: a packaged GUI build has no terminal.
	dataDir := services.DataDir()
	log, logErr := services.InitLogger(dataDir)
	defer func() {
		if err := services.CloseLogger(); err != nil {
			log.Error("close log file", "error", err)
		}
	}()
	if logErr != nil {
		// Non-fatal by design: the logger falls back to stderr and the app still
		// starts on a read-only data dir.
		log.Warn("file logging unavailable", "error", logErr)
	}
	log.Info("starting", "version", Version, "dataDir", dataDir)

	// The same build, as a tree rooted at its dist folder, for the loading
	// card's window to serve its page from (#97). Sub only fails on a bad
	// name; without it the card is unavailable and a start goes on without one.
	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		log.Error("embedded frontend build", "error", err)
	}

	app, err := NewApp(dataDir, bundledManifest, dist)
	if err != nil {
		// The bundled manifest is part of this build; if it does not validate,
		// the build is wrong and there is nothing sensible to show.
		log.Error("bundled manifest is invalid", "error", err)
		return
	}

	bg := design.WindowBackground
	err = wails.Run(&options.App{
		Title:            "Kapital Launcher",
		Width:            design.WindowWidth,
		Height:           design.WindowHeight,
		MinWidth:         design.WindowMinWidth,
		MinHeight:        design.WindowMinHeight,
		BackgroundColour: &options.RGBA{R: bg[0], G: bg[1], B: bg[2], A: 255},
		// The app draws its own header bar (docs/adr/0010-window-chrome.md).
		// Windows and Linux go frameless and get the header's own window
		// buttons; macOS keeps a hidden title bar so its native traffic
		// lights, full-screen and zoom stay the system's.
		Frameless: runtime.GOOS != "darwin",
		Mac:       &mac.Options{TitleBar: mac.TitleBarHidden()},
		// The wiki's screenshots are served from the app data dir (#141).
		AssetServer: &assetserver.Options{Assets: assets, Middleware: app.assetMiddleware},
		OnStartup:   app.startup,
		OnShutdown:  app.shutdown,
		Bind:        []any{app},
	})
	if err != nil {
		slog.Error("wails run", "error", err)
	}
}
