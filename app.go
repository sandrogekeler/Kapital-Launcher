package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"

	"kapital/backend/models"
	"kapital/backend/services"
	"kapital/backend/splashhost"
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
	ctx            context.Context
	manifest       models.Manifest
	settings       *services.SettingsService
	prism          *services.PrismService
	managed        *services.ManagedPrism
	status         *services.StatusService
	creator        *services.InstanceCreator
	wiki           *services.WikiService
	games          *services.GameTracker
	splash         *services.SplashCard
	live           *services.LiveLog
	stop           context.CancelFunc
	frontendErrors *services.FrontendErrorLog
	// runCtx is the context the background work runs under: cancelled in
	// shutdown, nil before startup.
	runCtx context.Context
	// openFolder shows a folder in the file manager; a test swaps it.
	openFolder func(string) error
	// emit, when set, takes the game:state events in place of the window; a
	// test sets it. emitInstall does the same for prism:install.
	emit        func(models.GameState)
	emitInstall func(models.PrismInstallProgress)
	// emitLive does the same for log:live (issue 155).
	emitLive func(models.LiveLogEvent)

	// previews are the developer previews that are on (#124, app_preview.go),
	// and previewStep the pause between the steps of a made-up Prism install.
	previews    services.PreviewSet
	previewStep time.Duration

	// What CopyRedactedLog needs: where the log is, who to mask, and the
	// clipboard. The clipboard is a field so a test needs no window.
	dataDir      string
	home, osUser string
	setClipboard func(ctx context.Context, text string) error

	// window is the launcher's window calls; a test swaps them.
	window windowCalls

	mu     sync.Mutex
	engine models.EngineInfo

	// seenMu guards what the app has seen of each chapter at run time.
	// ended is when a run the tracker followed last ended (app_chapters.go);
	// packVersions is the version each chapter's pack source served at the
	// last GetPackStates, for the loading card.
	seenMu       sync.Mutex
	ended        map[string]time.Time
	packVersions map[string]string
}

// NewApp wires the services. The manifest is parsed and validated here so a
// bad bundled manifest fails at startup rather than on the first click. dist
// is the embedded frontend build, which the loading card's window serves its
// page from (#97); nil makes the card unavailable and a start goes on without
// it.
func NewApp(dataDir string, manifest []byte, dist fs.FS) (*App, error) {
	m, err := services.ParseManifest(manifest)
	if err != nil {
		return nil, err
	}
	prism := services.NewPrismService(runtime.GOOS)
	managed := services.NewManagedPrism(dataDir, runtime.GOOS, runtime.GOARCH)
	prism.UseManaged(managed)
	// An unknown home is only a value the redactor then skips.
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Warn("home directory", "error", err)
	}
	a := &App{
		dataDir:        dataDir,
		home:           home,
		osUser:         services.OSUserName(),
		setClipboard:   wailsrt.ClipboardSetText,
		window:         wailsWindowCalls(),
		manifest:       m,
		settings:       services.NewSettingsService(dataDir),
		prism:          prism,
		managed:        managed,
		status:         services.NewStatusService(),
		creator:        services.NewInstanceCreator(dataDir),
		wiki:           services.NewWikiService(dataDir, m.Wiki.BaseURL),
		openFolder:     services.OpenFolder,
		frontendErrors: services.NewFrontendErrorLog(),
		previewStep:    previewInstallStep,
		live:           services.NewLiveLog(),
	}
	a.splash = a.newSplashCard(dist, splashhost.New)
	// Each phase change of a launched game is an event the frontend listens
	// for; before the window is up there is nobody to tell. The card sees the
	// phase first: a run that ends brings the launcher back before its event
	// says the card is gone.
	a.games = services.NewGameTracker(dataDir, a.onGameState)
	a.games.UseRedactor(a.redactor)
	return a, nil
}

// GetAppVersion returns the version stamped into this build.
func (a *App) GetAppVersion() (string, error) {
	return Version, nil
}

// GetManifest returns the chapter list and everything the UI renders from it.
func (a *App) GetManifest() (models.Manifest, error) {
	return a.manifest, nil
}

// GetInstances reports which chapters' Prism instances exist, read fresh from
// disk under the resolved Prism root. It stats one file per chapter and reads
// one key from Prism's config; see services/instances.go for exactly what.
// A chapter under a preview that fakes its instance (#124) is reported as the
// preview says.
func (a *App) GetInstances() (models.InstanceReport, error) {
	report, err := a.realInstances()
	if err != nil {
		return report, err
	}
	return a.previewInstances(report), nil
}

// realInstances is the read of the disk itself, which every action that
// resolves an instance folder uses whatever a preview shows the view.
func (a *App) realInstances() (models.InstanceReport, error) {
	settings, err := a.settings.Load()
	if err != nil {
		return models.InstanceReport{}, err
	}
	return a.prism.Instances(settings, a.realEngine(), a.manifest.Chapters), nil
}

// GetPackStates says, per chapter, whether the installed pack is the one its
// source serves now (#71): the source's pack.toml is fetched (the manifest's
// URL rules or the loopback rule, bounded) and its hash compared with the one
// packwiz-installer recorded at the last sync. Called when the instances are
// read, never on a timer. A chapter under a preview that fakes its pack (#124)
// is answered by the preview, and its source is not fetched.
func (a *App) GetPackStates() ([]models.PackState, error) {
	settings, err := a.settings.Load()
	if err != nil {
		return nil, err
	}
	report := a.prism.Instances(settings, a.realEngine(), a.manifest.Chapters)
	states := make([]models.PackState, 0, len(a.manifest.Chapters))
	for _, chapter := range a.manifest.Chapters {
		if state, ok := a.previewPackState(chapter.ID); ok {
			states = append(states, state)
			continue
		}
		state := a.creator.PackState(a.context(), chapter, report, settings.PackOverrides[chapter.ID])
		a.notePackVersion(state)
		states = append(states, state)
	}
	return states, nil
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
	if err := a.refuseUnderPreview(chapter); err != nil {
		return models.InstanceReport{}, err
	}
	engine := a.realEngine()
	if a.games.Active(chapterID) {
		return models.InstanceReport{}, fmt.Errorf("%s is starting or running; close the game first", chapter.Name)
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

// LaunchChapter starts the chapter's Prism instance, joining its server when
// it has one, and follows the game from there (#44). The chapter id is looked
// up in the validated manifest, so the instance id and address that reach
// Prism are the manifest's, never the caller's. A chapter whose game is
// already starting or running is refused.
func (a *App) LaunchChapter(chapterID string) error {
	chapter, ok := a.chapter(chapterID)
	if !ok {
		return fmt.Errorf("no chapter %q", chapterID)
	}
	if a.games.Active(chapterID) {
		return fmt.Errorf("%s is already starting or running", chapter.Name)
	}
	// A preview of the chapter ends here: the real run's events replace it.
	a.endChapterPreview(chapter.ID)
	// A Prism still up on the last start's hidden console is closed first, or
	// the new Prism would hand this launch to it (ADR-0012, amendment).
	a.games.CloseConsole(chapter.ID)
	settings, err := a.settings.Load()
	if err != nil {
		return err
	}
	engine := a.realEngine()
	req := models.LaunchRequest{
		InstanceID: chapter.Instance.ID,
		Profile:    settings.ProfileName,
		// The detected engine's root: the configured one for the player's own
		// Prism, the managed root for the launcher's copy.
		Root:   engine.Root,
		Server: joinAddress(chapter, settings),
	}
	// The game log as it is before Prism runs, so the log of an earlier start
	// is not taken for this one.
	instanceDir := a.instanceDir(settings, engine, chapter)
	// An instance made with the pack sync's window gets the headless command
	// before Prism reads it (#95).
	a.updatePreLaunch(chapterID, instanceDir)
	a.seedLogRules(engine)
	before := services.SnapshotGameLog(instanceDir)
	// Prism's own log likewise, for a launch step that fails before the game
	// (#103). The instance folder is <root>/instances/<id>, so the engine's
	// data root is what the log is under.
	prismRoot := a.prism.DataRoot(settings, engine)
	prismLog := services.SnapshotPrismLog(prismRoot)
	// The splash: a card in a window of its own opens before Prism starts and
	// the launcher steps aside (#97). Where the game's window can be held, it
	// is held until the handover (#43); a card that could not open holds
	// nothing, because a hidden game with no card would show nothing at all.
	splash := a.beginSplash(chapter, settings)
	// The window may have been closed while the card opened, which can take a
	// while: never start Prism from a process that is exiting.
	if a.trackContext().Err() != nil {
		if splash {
			a.splash.Leave()
		}
		return errors.New("the launcher is closing")
	}
	startedAt := time.Now()
	proc, err := a.prism.Launch(a.context(), engine, req)
	if err != nil {
		slog.Error("launch", "chapter", chapterID, "error", err)
		if splash {
			a.splash.Leave()
		}
		return err
	}
	slog.Info("launched", "chapter", chapterID, "instance", req.InstanceID, "server", req.Server != "")
	// The tracker follows the start on its own; a refusal here can only be a
	// second Play racing the first, and Prism has been started either way.
	track := services.TrackRequest{
		ChapterID:   chapterID,
		InstanceDir: instanceDir,
		Prism:       services.WatchPrism(proc),
		PrismExe:    filepath.Base(engine.Executable),
		StartedAt:   startedAt,
		Before:      before,
		PrismRoot:   prismRoot,
		PrismLog:    prismLog,
		HoldWindow:  splash && a.splash.HoldsGameWindow(),
	}
	if splash {
		// On Windows the card closes once the game has the foreground, not
		// before; macOS closes it at the game's window phase (Observe).
		track.OnHandover = func() { a.splash.Handover(chapterID) }
	}
	if err := a.games.Track(a.trackContext(), track); err != nil {
		slog.Warn("track game", "chapter", chapterID, "error", err)
	}
	return nil
}

// StopGame ends the chapter's run at once: its game, or the Prism the launcher
// started when there is no game yet, by the tracker and only for a run
// the launcher itself followed. The chapter id is looked up in the validated
// manifest like a launch's. The run finishes through its usual path, so the
// loading card, if it is up, hears of it as it does of a crash and stays with
// the reason. Returns the chapter's state as it is after the request. A
// chapter with no run in progress is an error.
func (a *App) StopGame(chapterID string) (models.GameState, error) {
	chapter, ok := a.chapter(chapterID)
	if !ok {
		return models.GameState{}, fmt.Errorf("no chapter %q", chapterID)
	}
	// Stop on a previewed run (#124) ends the preview, in a stopped state
	// of its own, and never reaches the tracker or a process.
	if s, ok := a.stopPreview(chapter); ok {
		return s, nil
	}
	if err := a.games.Stop(chapter.ID); err != nil {
		slog.Info("stop game", "chapter", chapter.ID, "error", err)
		if errors.Is(err, services.ErrNoGameToStop) {
			return models.GameState{}, fmt.Errorf("%s has no game to stop", chapter.Name)
		}
		return models.GameState{}, err
	}
	s := a.games.Latest(chapter.ID)
	s.Splash = a.splash.Showing(chapter.ID)
	return s, nil
}

// GetGameStates returns where every chapter's game is: each chapter's latest
// state, idle for one this run has not launched. The same states arrive as
// game:state events while a start is followed (#44), and carry the same
// splash flag and estimate (#43).
func (a *App) GetGameStates() ([]models.GameState, error) {
	states := make([]models.GameState, 0, len(a.manifest.Chapters))
	for _, c := range a.manifest.Chapters {
		states = append(states, a.latestGame(c.ID))
	}
	return states, nil
}

// LeaveSplash closes the loading card and brings the launcher's window back
// (#43, #97), which is what the card's own Back to launcher does. The game
// keeps starting and appears at the reload as usual. With no card up it does
// nothing. A game:state event with splash false follows at once, for the
// chapter whose card it was.
func (a *App) LeaveSplash() error {
	a.splash.Leave()
	return nil
}

// GetSettings returns the persisted settings, or defaults on a fresh install,
// with what the loading splash comes to on this OS (LoadingSplashAvailable
// and LoadingSplashOn, derived here and never stored).
func (a *App) GetSettings() (models.AppSettings, error) {
	settings, err := a.settings.Load()
	if err != nil {
		return settings, err
	}
	// A choice the manifest no longer lists is not handed back, so a save of
	// something else cannot write it again.
	settings.ServerChoices = services.PruneServerChoices(a.manifest.Chapters, settings.ServerChoices)
	return services.WithLoadingSplash(runtime.GOOS, settings), nil
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
	// A server choice is a label from the chapter's own list (issue 151).
	if err := services.ValidateServerChoices(a.manifest.Chapters, settings.ServerChoices); err != nil {
		return err
	}
	if err := a.settings.Save(settings); err != nil {
		return err
	}
	a.recheckServers(before, settings)
	if !services.AffectsDetection(before, settings) {
		return nil
	}
	_, err = a.detectEngine()
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

// CopyRedactedLog puts the end of kapital-launcher.log on the clipboard for a
// bug report and returns how many lines went (#84). The last 256 KiB at most,
// from a whole line, run through a redactor for this player: the home path,
// the OS user name, the Prism profile name and every server address in the
// manifest. Nothing is written to disk, and a path or value is never taken
// from the frontend.
func (a *App) CopyRedactedLog() (int, error) {
	redactor, err := a.redactor()
	if err != nil {
		return 0, err
	}
	text, lines, err := services.RedactedLogTail(services.LogPath(a.dataDir), redactor, services.LogCopyBytes)
	if err != nil {
		return 0, err
	}
	if err := a.setClipboard(a.context(), text); err != nil {
		slog.Error("copy log", "error", err)
		return 0, fmt.Errorf("copy log: %w", err)
	}
	slog.Info("log copied", "lines", lines)
	return lines, nil
}

// GetServerStatus pings the chapter's server now and returns the result. The
// same result is also emitted as a server:status event. A chapter with no
// server returns an offline, checked status rather than an error.
func (a *App) GetServerStatus(chapterID string) (models.ServerStatus, error) {
	chapter, ok := a.chapter(chapterID)
	if !ok {
		return models.ServerStatus{}, fmt.Errorf("no chapter %q", chapterID)
	}
	return a.checkServer(chapter), nil
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
// running instance is refused rather than raced with the game, on the
// tracker's word or on the game log's (refuseIfRunning).
func (a *App) SaveChapterSettings(chapterID string, settings models.ChapterSettings) (models.ChapterSettingsInfo, error) {
	if chapter, ok := a.chapter(chapterID); ok {
		if err := a.refuseUnderPreview(chapter); err != nil {
			return models.ChapterSettingsInfo{}, err
		}
	}
	chapter, cfg, err := a.chapterInstance(chapterID)
	if err != nil {
		return models.ChapterSettingsInfo{}, err
	}
	machine := services.MachineMemoryMB()
	if err := services.ValidateChapterSettings(settings, machine); err != nil {
		return models.ChapterSettingsInfo{}, fmt.Errorf("chapter settings: %w", err)
	}
	if err := a.refuseIfRunning(chapter, filepath.Dir(cfg)); err != nil {
		return models.ChapterSettingsInfo{}, err
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

// OpenInstanceFolder shows a chapter's instance folder in the file manager
// (#85), to reach a crash report, a screenshot or a config. The folder is the
// one chapterInstance resolves: the caller names a chapter and never a path,
// and an instance that is not there is refused.
func (a *App) OpenInstanceFolder(chapterID string) error {
	chapter, cfg, err := a.chapterInstance(chapterID)
	if err != nil {
		return err
	}
	if err := a.openFolder(filepath.Dir(cfg)); err != nil {
		slog.Error("open instance folder", "chapter", chapter.ID, "error", err)
		return fmt.Errorf("could not open the %s folder: %w", chapter.Name, err)
	}
	return nil
}

// GetWikiPages returns the wiki's pages for the "From the wiki" panel (#58):
// fetched from the manifest's wiki host once per start, cached in the app
// data dir, or read from that cache offline. An error means neither was
// possible, and the panel keeps the manifest's teaser.
func (a *App) GetWikiPages() ([]models.WikiPage, error) {
	return a.wiki.Pages(a.context())
}

// GetWikiShots returns the wiki's screenshots for the chapter art (#141):
// listed by the lore export, downloaded from the wiki host once per start and
// cached, each served to the page at its Src. It waits for the downloads; an
// error means the export itself could not be had, and the bundled art stays.
func (a *App) GetWikiShots() ([]models.WikiShot, error) {
	return a.wiki.Shots(a.context())
}

// assetMiddleware serves the cached wiki art ahead of the embedded build
// (main.go's AssetServer). Unexported, so Wails does not bind it.
func (a *App) assetMiddleware(next http.Handler) http.Handler {
	return a.wiki.ArtMiddleware(next)
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

// LogFrontendError writes an error caught in the view to the launcher's own
// log, since a packaged build has no console. The kind, the text lengths and
// the number written per run are bounded in services.FrontendErrorLog.
func (a *App) LogFrontendError(kind, message, stack string) error {
	return a.frontendErrors.Log(kind, message, stack)
}
