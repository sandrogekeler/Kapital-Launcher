package main

import (
	"fmt"
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

// chapterRunning is whether the chapter's game is running: the tracker says so
// for a start this app made, and the game log's recent activity covers a game
// started from Prism itself.
func (a *App) chapterRunning(chapter models.Chapter, instanceDir string) bool {
	return a.games.Active(chapter.ID) || services.InstanceRunning(instanceDir, time.Now())
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
		Running:         a.chapterRunning(chapter, filepath.Dir(cfg)),
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
