package main

import (
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

// updatePreLaunch brings an instance the launcher made up to its current
// pre-launch command before a launch (#95, ADR-2's fourth amendment). It
// refuses nothing: a failure or a command that is not the launcher's own is
// logged, never the command itself, and the launch goes on. An instance whose
// game looks to be running is left alone, as a settings save is (ADR-2, second
// amendment); Prism could be writing the file.
func (a *App) updatePreLaunch(chapterID, instanceDir string) {
	if instanceDir == "" {
		return
	}
	if services.InstanceRunning(instanceDir, time.Now()) {
		// A game that closed less than a minute ago looks the same; the next
		// Play catches up.
		slog.Info("pre-launch command left as it is", "chapter", chapterID, "reason", "the game looks to be running")
		return
	}
	result, err := services.RewritePreLaunchCommand(filepath.Join(instanceDir, "instance.cfg"))
	switch {
	case err != nil:
		slog.Warn("pre-launch command not updated", "chapter", chapterID, "error", err)
	case result == services.PreLaunchRewritten:
		slog.Info("pre-launch command updated", "chapter", chapterID, "headless", true)
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
// and is only ever checked fresh, at a write.
func (a *App) chapterRunning(chapter models.Chapter, instanceDir string) (tracker, guess bool) {
	return a.games.Active(chapter.ID), services.InstanceRunning(instanceDir, time.Now())
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
