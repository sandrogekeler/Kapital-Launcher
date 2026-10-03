package main

import (
	"fmt"

	"kapital/backend/models"
	"kapital/backend/services"
)

// redactor is the one that masks what identifies this player: the home path,
// the OS user name, the Prism profile name and every server address in the
// manifest. A copy of the launcher's log and a run report's game log both go
// through it, built from the settings as they are when asked.
func (a *App) redactor() (*services.Redactor, error) {
	settings, err := a.settings.Load()
	if err != nil {
		return nil, err
	}
	servers := make([]string, 0, len(a.manifest.Chapters))
	for _, c := range a.manifest.Chapters {
		if c.Server != nil {
			servers = append(servers, c.Server.Address)
		}
	}
	return services.NewRedactor(a.home, settings.ProfileName, servers, a.osUser), nil
}

// GetRunReport returns what the launcher knows of a chapter's latest run, now
// or ended: its phases, the redacted end of the game's log and the name of its
// crash report, for the view that shows how a start failed or the game
// crashed (ADR-2, sixth amendment). The chapter id is looked up in the
// manifest and the paths are the tracker's own, so nothing the caller sends
// names a file. It is read when asked and kept nowhere. A chapter this app
// run has not launched is refused.
func (a *App) GetRunReport(chapterID string) (models.RunReport, error) {
	chapter, ok := a.chapter(chapterID)
	if !ok {
		return models.RunReport{}, fmt.Errorf("no chapter %q", chapterID)
	}
	report, err := a.runReport(chapter.ID)
	if err != nil {
		return models.RunReport{}, fmt.Errorf("%s: %w", chapter.Name, err)
	}
	report.Game.Splash = a.splash.Showing(chapter.ID)
	return report, nil
}
