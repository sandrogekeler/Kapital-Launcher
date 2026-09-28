package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"

	"kapital/backend/models"
	"kapital/backend/services"
)

// App is the one struct Wails binds. Every exported method on it is callable
// from the frontend through the generated bindings in frontend/wailsjs/, and
// every one returns (T, error) so the frontend has a rejection to handle.
//
// The rules that hold on every method (agent_docs/SECURITY_CHECKLIST.md):
// a value from the frontend or a manifest is validated before it becomes an
// argument, a path or a URL; Prism is called with an argument array, never a
// shell string; and nothing here reads, copies or logs Prism's account data.
type App struct {
	ctx      context.Context
	manifest models.Manifest
	settings *services.SettingsService
	prism    *services.PrismService
	status   *services.StatusService
	stop     context.CancelFunc

	mu     sync.Mutex
	engine models.EngineInfo
}

// NewApp wires the services. The manifest is parsed and validated here so a
// bad bundled manifest fails at startup rather than on the first click.
func NewApp(dataDir string, manifest []byte) (*App, error) {
	m, err := services.ParseManifest(manifest)
	if err != nil {
		return nil, err
	}
	return &App{
		manifest: m,
		settings: services.NewSettingsService(dataDir),
		prism:    services.NewPrismService(runtime.GOOS),
		status:   services.NewStatusService(),
	}, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if _, err := a.RefreshEngine(); err != nil {
		slog.Warn("engine detection", "error", err)
	}
	// The status ticker lives as long as the window. Each result is an event
	// the frontend listens for; a chapter's line updates without asking.
	runCtx, cancel := context.WithCancel(ctx)
	a.stop = cancel
	go a.status.Run(runCtx, a.manifest.Chapters, func(s models.ServerStatus) {
		wailsrt.EventsEmit(a.ctx, services.EventServerStatus, s)
	})
}

func (a *App) shutdown(context.Context) {
	if a.stop != nil {
		a.stop()
	}
}

// GetAppVersion returns the version stamped into this build.
func (a *App) GetAppVersion() (string, error) {
	return Version, nil
}

// GetManifest returns the chapter list and everything the UI renders from it.
func (a *App) GetManifest() (models.Manifest, error) {
	return a.manifest, nil
}

// GetEngine returns what is known about the Prism install, from the last
// detection. RefreshEngine re-runs detection.
func (a *App) GetEngine() (models.EngineInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.engine, nil
}

// RefreshEngine re-detects Prism with the current settings and returns the
// result. Called on startup and after the settings change.
func (a *App) RefreshEngine() (models.EngineInfo, error) {
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

// GetInstances reports which chapters' Prism instances exist, read fresh from
// disk under the resolved Prism root. It stats one file per chapter and reads
// one key from Prism's config; see services/instances.go for exactly what.
func (a *App) GetInstances() (models.InstanceReport, error) {
	settings, err := a.settings.Load()
	if err != nil {
		return models.InstanceReport{}, err
	}
	engine, err := a.GetEngine()
	if err != nil {
		return models.InstanceReport{}, err
	}
	return a.prism.Instances(settings, engine, a.manifest.Chapters), nil
}

// LaunchChapter starts the chapter's Prism instance, joining its server when
// it has one. The chapter id is looked up in the validated manifest, so the
// instance id and address that reach Prism are the manifest's, never the
// caller's.
func (a *App) LaunchChapter(chapterID string) error {
	chapter, ok := a.chapter(chapterID)
	if !ok {
		return fmt.Errorf("no chapter %q", chapterID)
	}
	settings, err := a.settings.Load()
	if err != nil {
		return err
	}
	engine, err := a.GetEngine()
	if err != nil {
		return err
	}
	req := models.LaunchRequest{
		InstanceID: chapter.Instance.ID,
		Profile:    settings.ProfileName,
		Root:       settings.PrismRoot,
	}
	if chapter.Server != nil && chapter.Server.JoinOnLaunch {
		req.Server = chapter.Server.Address
	}
	if err := a.prism.Launch(a.context(), engine, req); err != nil {
		slog.Error("launch", "chapter", chapterID, "error", err)
		return err
	}
	slog.Info("launched", "chapter", chapterID, "instance", req.InstanceID, "server", req.Server != "")
	return nil
}

// GetSettings returns the persisted settings, or defaults on a fresh install.
func (a *App) GetSettings() (models.AppSettings, error) {
	return a.settings.Load()
}

// SaveSettings validates and persists, then re-detects the engine when a field
// detection reads has changed. Detection runs Prism's --version, so a save
// that only records the open chapter must not reach it (#6).
func (a *App) SaveSettings(settings models.AppSettings) error {
	before, err := a.settings.Load()
	if err != nil {
		// An unreadable file is about to be replaced; re-detect to be safe.
		before = models.AppSettings{}
		slog.Warn("settings before save", "error", err)
	}
	if err := a.settings.Save(settings); err != nil {
		return err
	}
	if !services.AffectsDetection(before, settings) {
		return nil
	}
	_, err = a.RefreshEngine()
	return err
}

// GetServerStatus pings the chapter's server now and returns the result. The
// same result is also emitted as a server:status event. A chapter with no
// server returns an offline, checked status rather than an error.
func (a *App) GetServerStatus(chapterID string) (models.ServerStatus, error) {
	chapter, ok := a.chapter(chapterID)
	if !ok {
		return models.ServerStatus{}, fmt.Errorf("no chapter %q", chapterID)
	}
	status := a.status.Check(a.context(), chapter)
	if a.ctx != nil {
		wailsrt.EventsEmit(a.ctx, services.EventServerStatus, status)
	}
	return status, nil
}

// OpenChapterWiki opens the chapter's wiki page in the system browser. The URL
// is built from the validated manifest, so the frontend never hands one over.
func (a *App) OpenChapterWiki(chapterID string) error {
	chapter, ok := a.chapter(chapterID)
	if !ok {
		return fmt.Errorf("no chapter %q", chapterID)
	}
	return a.openURL(services.WikiURL(a.manifest, chapter))
}

// OpenExternal opens a web address in the system browser. http and https
// only; anything else is refused before it reaches the OS.
func (a *App) OpenExternal(raw string) error {
	checked, err := services.ExternalURL(raw)
	if err != nil {
		return err
	}
	return a.openURL(checked)
}

func (a *App) openURL(url string) error {
	if a.ctx == nil {
		return errors.New("window is not ready")
	}
	wailsrt.BrowserOpenURL(a.ctx, url)
	return nil
}

func (a *App) chapter(id string) (models.Chapter, bool) {
	for _, c := range a.manifest.Chapters {
		if c.ID == id {
			return c, true
		}
	}
	return models.Chapter{}, false
}

// context is the Wails context once the window is up, or Background before
// it: detection during NewApp must not depend on the window.
func (a *App) context() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
