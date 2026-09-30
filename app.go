package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

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
	managed  *services.ManagedPrism
	status   *services.StatusService
	creator  *services.InstanceCreator
	wiki     *services.WikiService
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
	prism := services.NewPrismService(runtime.GOOS)
	managed := services.NewManagedPrism(dataDir, runtime.GOOS, runtime.GOARCH)
	prism.UseManaged(managed)
	return &App{
		manifest: m,
		settings: services.NewSettingsService(dataDir),
		prism:    prism,
		managed:  managed,
		status:   services.NewStatusService(),
		creator:  services.NewInstanceCreator(dataDir),
		wiki:     services.NewWikiService(dataDir, m.Wiki.BaseURL),
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

// InstallChapter writes the chapter's Prism instance into the instances folder
// GetInstances resolves, and returns that report read again (#24). The chapter
// id is looked up in the validated manifest, so the instance id and pack URL
// are the manifest's, unless settings name a local pack for it. What is written, and the one-folder rule, is
// services/packinstance.go's; the pack itself downloads on the first Play.
func (a *App) InstallChapter(chapterID string) (models.InstanceReport, error) {
	chapter, ok := a.chapter(chapterID)
	if !ok {
		return models.InstanceReport{}, fmt.Errorf("no chapter %q", chapterID)
	}
	engine, err := a.GetEngine()
	if err != nil {
		return models.InstanceReport{}, err
	}
	if !engine.Found {
		return models.InstanceReport{}, services.ErrPrismNotFound
	}
	settings, err := a.settings.Load()
	if err != nil {
		return models.InstanceReport{}, err
	}
	report := a.prism.Instances(settings, engine, a.manifest.Chapters)
	// A developer's local packwiz serve, from settings.json (#41); the
	// creator holds it to loopback, whatever the settings file says.
	override := settings.PackOverrides[chapter.ID]
	if err := a.creator.Install(a.context(), chapter, report, override); err != nil {
		slog.Error("install chapter", "chapter", chapterID, "error", err)
		return report, err
	}
	slog.Info("installed", "chapter", chapterID, "instance", chapter.Instance.ID, "local pack", override != "")
	return a.GetInstances()
}

// GetPrismRelease reads Prism's latest release: what installing Prism would
// download, and whether the launcher-managed copy has an update. Nothing is
// downloaded; the approval card shows this before InstallPrism is called.
func (a *App) GetPrismRelease() (models.PrismRelease, error) {
	rel, err := a.managed.Latest(a.context())
	if err != nil {
		return rel, err
	}
	// An update is only worth offering for the Prism the launcher runs.
	if engine, err := a.GetEngine(); err != nil || engine.Source != "managed" {
		rel.UpdateAvailable = false
	}
	return rel, nil
}

// InstallPrism installs or updates the launcher-managed Prism from Prism's
// latest release, after the player approved it. The release is read again
// here rather than taken from the caller, so what is downloaded is always
// what Prism published. Progress arrives as prism:install events; detection
// re-runs once it is in place.
func (a *App) InstallPrism() error {
	emit := func(p models.PrismInstallProgress) {
		if a.ctx != nil {
			wailsrt.EventsEmit(a.ctx, services.EventPrismInstall, p)
		}
	}
	rel, err := a.managed.Latest(a.context())
	if err != nil {
		emit(models.PrismInstallProgress{Phase: "failed", Error: err.Error()})
		return err
	}
	if err := a.managed.Install(a.context(), rel, emit); err != nil {
		slog.Error("install prism", "version", rel.Version, "error", err)
		return err
	}
	_, err = a.RefreshEngine()
	return err
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
		// The detected engine's root: the configured one for the player's own
		// Prism, the managed root for the launcher's copy.
		Root: engine.Root,
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
//
// A Prism executable that changed must exist as a file, so the settings
// screen can say so under the field rather than detection quietly falling
// past it (#5). Only when it changed: every save writes the whole settings,
// and a program removed since must not stop the open chapter being saved.
func (a *App) SaveSettings(settings models.AppSettings) error {
	before, err := a.settings.Load()
	if err != nil {
		// An unreadable file is about to be replaced; re-detect to be safe.
		before = models.AppSettings{}
		slog.Warn("settings before save", "error", err)
	}
	if services.ExecutableChanged(before, settings) {
		if err := a.prism.CheckExecutable(settings.PrismExecutable); err != nil {
			return fmt.Errorf("settings: %w", err)
		}
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

// ChoosePrismExecutable opens the native file picker for the Prism program and
// returns the pick, or "" when the player cancelled. Nothing is saved here: the
// frontend puts the path into the field and commits it through SaveSettings,
// so a path always arrives by the same validated route, picked or typed (#5).
// On macOS the pick is the .app bundle and resolves to its executable.
func (a *App) ChoosePrismExecutable() (string, error) {
	if a.ctx == nil {
		return "", errors.New("window is not ready")
	}
	opts := wailsrt.OpenDialogOptions{Title: "Choose the Prism Launcher program"}
	switch runtime.GOOS {
	case "windows":
		opts.Filters = []wailsrt.FileFilter{{DisplayName: "Programs (*.exe)", Pattern: "*.exe"}}
	case "darwin":
		opts.Filters = []wailsrt.FileFilter{{DisplayName: "Applications (*.app)", Pattern: "*.app"}}
	}
	picked, err := wailsrt.OpenFileDialog(a.ctx, opts)
	if err != nil {
		return "", fmt.Errorf("choose prism executable: %w", err)
	}
	return services.ResolvePrismExecutable(runtime.GOOS, picked), nil
}

// ChoosePrismRoot opens the native folder picker for a Prism data root and
// returns the pick, or "" when the player cancelled. Saved by the frontend
// through SaveSettings, as ChoosePrismExecutable's pick is.
func (a *App) ChoosePrismRoot() (string, error) {
	if a.ctx == nil {
		return "", errors.New("window is not ready")
	}
	picked, err := wailsrt.OpenDirectoryDialog(a.ctx, wailsrt.OpenDialogOptions{
		Title:                "Choose the Prism data folder",
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", fmt.Errorf("choose prism root: %w", err)
	}
	return strings.TrimSpace(picked), nil
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

// GetChapterSettings reads a chapter's memory and JVM preset from its
// instance, with what the panel needs around them (#36). The chapter id is
// looked up in the manifest; the instance must exist.
func (a *App) GetChapterSettings(chapterID string) (models.ChapterSettingsInfo, error) {
	chapter, cfg, err := a.chapterInstance(chapterID)
	if err != nil {
		return models.ChapterSettingsInfo{}, err
	}
	machine := services.MachineMemoryMB()
	settings, err := services.ReadChapterSettings(cfg, machine)
	if err != nil {
		return models.ChapterSettingsInfo{}, err
	}
	return a.chapterSettingsInfo(chapter, cfg, settings, machine), nil
}

// SaveChapterSettings writes a chapter's memory and JVM preset into its
// instance.cfg, those keys and nothing else, and reads the result back. The
// value is held to the fixed preset list and the machine's memory, and a
// running instance is refused rather than raced with the game.
func (a *App) SaveChapterSettings(chapterID string, settings models.ChapterSettings) (models.ChapterSettingsInfo, error) {
	chapter, cfg, err := a.chapterInstance(chapterID)
	if err != nil {
		return models.ChapterSettingsInfo{}, err
	}
	machine := services.MachineMemoryMB()
	if err := services.ValidateChapterSettings(settings, machine); err != nil {
		return models.ChapterSettingsInfo{}, fmt.Errorf("chapter settings: %w", err)
	}
	if services.InstanceRunning(filepath.Dir(cfg), time.Now()) {
		return models.ChapterSettingsInfo{}, fmt.Errorf("%s looks to be running; close the game first", chapter.Name)
	}
	if err := services.WriteChapterSettings(cfg, settings); err != nil {
		slog.Error("save chapter settings", "chapter", chapterID, "error", err)
		return models.ChapterSettingsInfo{}, err
	}
	slog.Info("chapter settings saved", "chapter", chapterID, "maxMemoryMb", settings.MaxMemoryMB, "jvm", settings.JVM)
	saved, err := services.ReadChapterSettings(cfg, machine)
	if err != nil {
		return models.ChapterSettingsInfo{}, err
	}
	return a.chapterSettingsInfo(chapter, cfg, saved, machine), nil
}

// chapterInstance resolves a chapter id to its instance.cfg under the
// instances folder GetInstances resolves, refusing a chapter whose instance
// is not there.
func (a *App) chapterInstance(chapterID string) (models.Chapter, string, error) {
	chapter, ok := a.chapter(chapterID)
	if !ok {
		return models.Chapter{}, "", fmt.Errorf("no chapter %q", chapterID)
	}
	report, err := a.GetInstances()
	if err != nil {
		return models.Chapter{}, "", err
	}
	if !report.Present[chapterID] {
		return models.Chapter{}, "", fmt.Errorf("%s is not installed", chapter.Name)
	}
	return chapter, filepath.Join(report.Dir, chapter.Instance.ID, "instance.cfg"), nil
}

func (a *App) chapterSettingsInfo(chapter models.Chapter, cfg string, settings models.ChapterSettings, machine int) models.ChapterSettingsInfo {
	info := models.ChapterSettingsInfo{
		ChapterID:       chapter.ID,
		Settings:        settings,
		MachineMemoryMB: machine,
		PrismDefaultMB:  services.PrismDefaultMaxMB(machine),
		Presets:         services.PresetNames(),
		Running:         services.InstanceRunning(filepath.Dir(cfg), time.Now()),
	}
	if chapter.Pack.MemoryGB != nil {
		info.PackMemoryMB = *chapter.Pack.MemoryGB * 1024
	}
	return info
}

// GetWikiPages returns the wiki's pages for the "From the wiki" panel (#58):
// fetched from the manifest's wiki host once per start, cached in the app
// data dir, or read from that cache offline. An error means neither was
// possible, and the panel keeps the manifest's teaser.
func (a *App) GetWikiPages() ([]models.WikiPage, error) {
	return a.wiki.Pages(a.context())
}

// OpenWikiPage opens one of the pages GetWikiPages returned in the system
// browser. The URL must be one of those, so the bridge picks a page and never
// names an address of its own.
func (a *App) OpenWikiPage(url string) error {
	if !a.wiki.Known(url) {
		return fmt.Errorf("not a wiki page this app listed: %q", url)
	}
	return a.openURL(url)
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
