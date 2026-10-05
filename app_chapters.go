package main

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"kapital/backend/models"
	"kapital/backend/services"
)

// instanceDir is the chapter's instance folder under the instances folder
// GetInstances resolves, or "" when that cannot be worked out.
func (a *App) instanceDir(settings models.AppSettings, engine models.EngineInfo, chapter models.Chapter) string {
	report := a.prism.Instances(settings, engine, []models.Chapter{chapter})
	if report.Dir == "" {
		return ""
	}
	return filepath.Join(report.Dir, chapter.Instance.ID)
}

// syncExe is the path of the launcher's sync copy for the pre-launch command
// (services.SyncCopy, issue 156), made or refreshed on the way, or "" when it
// cannot be made, which gives the packwiz command an instance ran before.
func (a *App) syncExe() string {
	if a.syncCopy == nil {
		return ""
	}
	return a.syncCopy.Path()
}

// updatePreLaunch brings an instance the launcher made up to its current
// pre-launch command before a launch (#95, ADR-2's fourth amendment): the
// headless sync, and from issue 156 the launcher's own sync copy (ninth
// amendment), made or refreshed here first, so the command names a copy of this
// build. It refuses nothing: a failure or a command that is not the launcher's
// own is logged, never the command itself, and the launch goes on. An instance
// whose game looks to be running is left alone, as a settings save is (ADR-2,
// second amendment); Prism could be writing the file. So is one while a Prism
// is open (issue 209, thirteenth amendment): it has the old command in memory
// and writes it back at the launch this Play hands it, so the rewrite is kept
// for a later Play. The launcher's own leftover Prism was closed by
// LaunchChapter before this ran, and one that would not close shows here as an
// open one.
func (a *App) updatePreLaunch(chapterID, instanceDir string) {
	if instanceDir == "" {
		return
	}
	if services.InstanceRunning(instanceDir, time.Now(), a.endedAt(chapterID)) {
		// A game that closed less than a minute ago looks the same; the next
		// Play catches up.
		slog.Info("pre-launch command left as it is", "chapter", chapterID, "reason", "the game looks to be running")
		return
	}
	if open, err := a.otherPrismOpen(chapterID, a.realEngine().Executable); err != nil {
		slog.Info("pre-launch command: could not tell whether Prism is open", "chapter", chapterID, "error", err)
	} else if open {
		slog.Info("pre-launch command left as it is", "chapter", chapterID, "reason", "Prism is open")
		return
	}
	result, err := services.RewritePreLaunchCommand(filepath.Join(instanceDir, "instance.cfg"), a.syncExe())
	switch {
	case err != nil:
		slog.Warn("pre-launch command not updated", "chapter", chapterID, "error", err)
	case result == services.PreLaunchRewritten:
		slog.Info("pre-launch command updated", "chapter", chapterID, "syncCopy", a.syncCopy != nil)
	case result == services.PreLaunchForeign:
		slog.Info("pre-launch command left as it is", "chapter", chapterID, "reason", "not the launcher's own command")
	}
}

// seedLogRules gives a managed Prism that was installed before its log rules
// were seeded those rules now, before it starts and reads them (#103, #106).
// A Prism the player installed is never written to.
func (a *App) seedLogRules(engine models.EngineInfo) {
	if engine.Source == "managed" {
		a.managed.SeedLogRules()
	}
}

// chapterRunning says whether the chapter's game is running and which of two
// answers said so. The tracker is exact for a start this app made. The guess
// (InstanceRunning) is the game log's recent activity, the last line of
// defence for a game started from Prism itself; it can be wrong either way
// and is only ever checked fresh, at a write. A log the tracker saw its run
// end with is not counted, so a game closed from the launcher frees the
// instance at once rather than a minute later.
func (a *App) chapterRunning(chapter models.Chapter, instanceDir string) (tracker, guess bool) {
	return a.games.Active(chapter.ID), services.InstanceRunning(instanceDir, time.Now(), a.endedAt(chapter.ID))
}

// noteGameEnd records when a run the tracker followed ended, for the guess.
func (a *App) noteGameEnd(s models.GameState) {
	switch s.Phase {
	case models.GamePhaseClosed, models.GamePhaseCrashed, models.GamePhaseFailed:
	default:
		return
	}
	a.seenMu.Lock()
	defer a.seenMu.Unlock()
	if a.ended == nil {
		a.ended = map[string]time.Time{}
	}
	a.ended[s.ChapterID] = time.Now()
}

// endedAt is when the chapter's last followed run ended, zero when none has
// in this app run.
func (a *App) endedAt(chapterID string) time.Time {
	a.seenMu.Lock()
	defer a.seenMu.Unlock()
	return a.ended[chapterID]
}

// notePackVersion keeps the version a chapter's pack source served, when it
// was read and names one.
func (a *App) notePackVersion(state models.PackState) {
	if !state.Checked || state.Version == "" {
		return
	}
	a.seenMu.Lock()
	defer a.seenMu.Unlock()
	if a.packVersions == nil {
		a.packVersions = map[string]string{}
	}
	a.packVersions[state.ChapterID] = state.Version
}

// withPackVersion is the chapter with the version its pack source served at
// the last read, which is what Play syncs to, in place of the manifest's. The
// manifest's stays when the source has not been read.
func (a *App) withPackVersion(chapter models.Chapter) models.Chapter {
	a.seenMu.Lock()
	defer a.seenMu.Unlock()
	if v, ok := a.packVersions[chapter.ID]; ok {
		chapter.Pack.Version = &v
	}
	return chapter
}

// refuseIfRunning is the guard of every write to an instance: nil when the
// game is closed, otherwise an error that says which answer refused. The
// tracker's is certain; the guess says what it saw and that a closed game
// only needs a moment, since the launcher's window never disables anything on it.
func (a *App) refuseIfRunning(chapter models.Chapter, instanceDir string) error {
	tracker, guess := a.chapterRunning(chapter, instanceDir)
	switch {
	case tracker:
		return errGameActive(chapter)
	case guess:
		return fmt.Errorf("%s's game log changed less than a minute ago: if the game is closed, try again in a moment", chapter.Name)
	}
	return nil
}

// refuseIfPrismOpen is the second guard of a write to instance.cfg, after
// refuseIfRunning (issue 209, ADR-2 thirteenth amendment): Prism keeps the
// file's settings in memory and writes them all back at a launch, on a save and
// on exit, so a key written while a Prism has the instance loaded comes back as it
// was. The Prism the launcher started for the chapter is closed first, with
// Stop's routine, and waited for; one that is still there refuses. Then any
// other Prism running from the engine's executable refuses, telling the player
// to close it: the launcher never ends a process it did not start. Where the
// processes cannot be listed the write goes on as it did before.
func (a *App) refuseIfPrismOpen(chapter models.Chapter, exe string) error {
	if !a.ownPrismGone(chapter.ID) {
		return fmt.Errorf("The Prism opened for %s did not close when asked; close it first, or the change is lost when Prism saves", chapter.Name)
	}
	open, err := a.otherPrismOpen(chapter.ID, exe)
	if err != nil {
		slog.Info("could not tell whether Prism is open", "chapter", chapter.ID, "error", err)
		return nil
	}
	if open {
		return errPrismOpen
	}
	return nil
}

// errPrismOpen is what a write refused for an open Prism says.
var errPrismOpen = errors.New("Prism is open; close it first, or the change is lost when Prism saves")

// ownPrismGone closes the Prism the launcher started for the chapter if it is
// still alive after its run, and says whether it is gone.
func (a *App) ownPrismGone(chapterID string) bool {
	switch {
	case a.closeOwnPrism != nil:
		return a.closeOwnPrism(chapterID)
	case a.games != nil:
		return a.games.CloseOwnPrism(chapterID)
	}
	return true
}

// otherPrismOpen says whether a Prism other than the chapter's own is running
// from the engine's executable. An error means it could not be told.
func (a *App) otherPrismOpen(chapterID, exe string) (bool, error) {
	switch {
	case a.prismOpenElsewhere != nil:
		return a.prismOpenElsewhere(chapterID, exe)
	case a.games != nil:
		return a.games.OtherPrismOpen(chapterID, exe)
	}
	return false, nil
}

func errGameActive(chapter models.Chapter) error {
	return fmt.Errorf("%s is starting or running; close the game first", chapter.Name)
}

// chapterInstance resolves a chapter id to its instance.cfg under the
// instances folder GetInstances resolves, refusing a chapter whose instance
// is not there.
func (a *App) chapterInstance(chapterID string) (models.Chapter, string, error) {
	chapter, ok := a.chapter(chapterID)
	if !ok {
		return models.Chapter{}, "", fmt.Errorf("no chapter %q", chapterID)
	}
	// The disk's own report, not a preview's (#124): a preview never decides
	// which folder is written.
	report, err := a.realInstances()
	if err != nil {
		return models.Chapter{}, "", err
	}
	if !report.Present[chapterID] {
		return models.Chapter{}, "", fmt.Errorf("%s is not installed", chapter.Name)
	}
	return chapter, filepath.Join(report.Dir, chapter.Instance.ID, "instance.cfg"), nil
}

func (a *App) chapterSettingsInfo(chapter models.Chapter, cfg string, settings models.ChapterSettings, machine int) models.ChapterSettingsInfo {
	tracker, guess := a.chapterRunning(chapter, filepath.Dir(cfg))
	info := models.ChapterSettingsInfo{
		ChapterID:       chapter.ID,
		Settings:        settings,
		MachineMemoryMB: machine,
		PrismDefaultMB:  services.PrismDefaultMaxMB(machine),
		Presets:         services.PresetNames(),
		Running:         tracker || guess,
	}
	if chapter.Pack.MemoryGB != nil {
		info.PackMemoryMB = *chapter.Pack.MemoryGB * 1024
	}
	return info
}

func (a *App) chapter(id string) (models.Chapter, bool) {
	for _, c := range a.manifest.Chapters {
		if c.ID == id {
			return c, true
		}
	}
	return models.Chapter{}, false
}
