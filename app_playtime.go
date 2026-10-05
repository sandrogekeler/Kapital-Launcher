package main

import (
	"log/slog"
	"path/filepath"

	"kapital/backend/models"
	"kapital/backend/services"
)

// GetPlayTime returns, for each manifest chapter whose instance is installed,
// the play time Prism has counted in its instance.cfg (issue 192, ADR-2's
// eleventh amendment): `totalTimePlayed` and `lastLaunchTime`, and nothing else
// of the file. The instances are the disk's own report, as chapterInstance
// resolves them, and the paths are built from the manifest's instance ids, so
// nothing the caller sends names a file. A chapter that is not installed is
// left out; one whose file cannot be read is left out as well and logged by
// chapter only. Read when the account page opens or the window regains focus,
// never on a timer.
func (a *App) GetPlayTime() ([]models.PlayTime, error) {
	report, err := a.realInstances()
	if err != nil {
		return nil, err
	}
	times := make([]models.PlayTime, 0, len(a.manifest.Chapters))
	for _, chapter := range a.manifest.Chapters {
		if !report.Present[chapter.ID] {
			continue
		}
		cfg := filepath.Join(report.Dir, chapter.Instance.ID, "instance.cfg")
		t, err := services.ReadPlayTime(cfg, chapter.ID)
		if err != nil {
			slog.Warn("play time not read", "chapter", chapter.ID, "error", err)
			continue
		}
		times = append(times, t)
	}
	return times, nil
}
