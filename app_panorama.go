package main

import (
	"log/slog"
	"path/filepath"

	"kapital/backend/models"
	"kapital/backend/services"
)

// GetPanoramas returns the title-screen panorama of each installed chapter that
// has one (issue 195): the six faces of the resources pack in the chapter's own
// instance, validated and cached by Go and served to the page at
// /panorama/<chapter>/panorama_<n>.png. A chapter that is not installed, or has
// no such pack, is left out and its page shows its own picture. The page asks on
// window focus and after an install or a run, never on a timer. Chapter paths
// come from the manifest and the instance report; nothing the caller sends
// names one.
func (a *App) GetPanoramas() ([]models.Panorama, error) {
	report, err := a.realInstances()
	if err != nil {
		slog.Error("panoramas: instances", "error", err)
		return nil, err
	}
	sources := make([]services.PanoramaSource, 0, len(a.manifest.Chapters))
	for _, chapter := range a.manifest.Chapters {
		dir := ""
		if report.Present[chapter.ID] {
			dir = filepath.Join(report.Dir, chapter.Instance.ID)
		}
		sources = append(sources, services.PanoramaSource{ChapterID: chapter.ID, InstanceDir: dir})
	}
	return a.panoramas.Refresh(sources), nil
}
