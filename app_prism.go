package main

import (
	"log/slog"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"

	"kapital/backend/models"
	"kapital/backend/services"
)

// The Prism the launcher runs: what detection found, re-detecting it, and
// getting or updating the launcher-managed copy (ADR-11). Each answer the view
// reads may come from a developer preview (#124, app_preview.go); the app's own
// work reads realEngine.

// GetEngine returns what is known about the Prism install, from the last
// detection. RefreshEngine re-runs detection. Under a Prism preview (#124) it
// answers with the synthetic engine; the app's own work reads realEngine.
func (a *App) GetEngine() (models.EngineInfo, error) {
	if info, ok := a.previewEngine(); ok {
		return info, nil
	}
	return a.realEngine(), nil
}

// realEngine is the Prism the last detection found, which every action of the
// app uses whatever a preview shows the view.
func (a *App) realEngine() models.EngineInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.engine
}

// RefreshEngine re-detects Prism with the current settings and returns the
// result. Called on startup and after the settings change. Under a Prism
// preview it runs nothing and answers with the synthetic engine.
func (a *App) RefreshEngine() (models.EngineInfo, error) {
	if info, ok := a.previewEngine(); ok {
		return info, nil
	}
	return a.detectEngine()
}

// detectEngine is the detection itself, and keeps the result.
func (a *App) detectEngine() (models.EngineInfo, error) {
	settings, err := a.settings.Load()
	if err != nil {
		return models.EngineInfo{}, err
	}
	info := a.prism.Detect(a.context(), settings)
	a.mu.Lock()
	a.engine = info
	a.mu.Unlock()
	slog.Info("engine", "found", info.Found, "source", info.Source, "version", info.Version)
	return info, nil
}

// GetPrismRelease reads Prism's latest release: what installing Prism would
// download, and whether the launcher-managed copy has an update. Nothing is
// downloaded; the approval card shows this before InstallPrism is called.
func (a *App) GetPrismRelease() (models.PrismRelease, error) {
	// Under a Prism preview (#124) the release is a made-up one: no request.
	if rel, ok := a.previewRelease(); ok {
		return rel, nil
	}
	rel, err := a.managed.Latest(a.context())
	if err != nil {
		return rel, err
	}
	// An update is only worth offering for the Prism the launcher runs, and
	// measured against the version it runs, which Prism's own updater may have
	// changed (#221).
	engine := a.realEngine()
	if engine.Source != "managed" {
		rel.UpdateAvailable = false
		return rel, nil
	}
	a.managed.Reconcile(a.context(), &rel, engine.Version)
	return rel, nil
}

// InstallPrism installs or updates the launcher-managed Prism from Prism's
// latest release, after the player approved it. The release is read again
// here rather than taken from the caller, so what is downloaded is always
// what Prism published. Progress arrives as prism:install events; detection
// re-runs once it is in place.
func (a *App) InstallPrism() error {
	// Under a Prism preview (#124) the install is a made-up one that fails: it
	// never reaches the installer, so nothing is downloaded or unpacked.
	if a.previews.Prism() != "" {
		return a.playPreviewInstall()
	}
	rel, err := a.managed.Latest(a.context())
	if err != nil {
		a.emitPrismInstall(models.PrismInstallProgress{Phase: "failed", Error: err.Error()})
		return err
	}
	if err := a.managed.Install(a.context(), rel, a.emitPrismInstall); err != nil {
		slog.Error("install prism", "version", rel.Version, "error", err)
		return err
	}
	_, err = a.detectEngine()
	return err
}

// emitPrismInstall is one prism:install event.
func (a *App) emitPrismInstall(p models.PrismInstallProgress) {
	if a.emitInstall != nil {
		a.emitInstall(p)
		return
	}
	if a.ctx != nil {
		wailsrt.EventsEmit(a.ctx, services.EventPrismInstall, p)
	}
}
