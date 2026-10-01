package main

import (
	"context"
	"errors"
	"log/slog"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"

	"kapital/backend/models"
	"kapital/backend/services"
)

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if _, err := a.RefreshEngine(); err != nil {
		slog.Warn("engine detection", "error", err)
	}
	// The status ticker lives as long as the window. Each result is an event
	// the frontend listens for; a chapter's line updates without asking.
	runCtx, cancel := context.WithCancel(ctx)
	a.stop = cancel
	a.runCtx = runCtx
	go a.status.Run(runCtx, a.manifest.Chapters, func(s models.ServerStatus) {
		wailsrt.EventsEmit(a.ctx, services.EventServerStatus, s)
	})
}

func (a *App) shutdown(context.Context) {
	if a.stop != nil {
		a.stop()
	}
}

// trackContext is the context the tracker's goroutines run under: cancelled in
// shutdown, Background before the window is up.
func (a *App) trackContext() context.Context {
	if a.runCtx != nil {
		return a.runCtx
	}
	return context.Background()
}

func (a *App) openURL(url string) error {
	if a.ctx == nil {
		return errors.New("window is not ready")
	}
	wailsrt.BrowserOpenURL(a.ctx, url)
	return nil
}

// context is the Wails context once the window is up, or Background before
// it: detection during NewApp must not depend on the window.
func (a *App) context() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
