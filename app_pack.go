package main

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"kapital/backend/models"
	"kapital/backend/services"
)

// The two words SetPackSource takes.
const (
	packSourcePublished = "published"
	packSourceDev       = "dev"
)

// SetPackSource points an installed chapter's instance at its published pack
// or at the developer's local one, and returns the instances read again, so
// the frontend has the new packUrl (ADR-2, seventh amendment; the way back
// from a dev pack that #41 left only as deleting the instance in Prism).
//
// The caller names a chapter and one of two words and never a URL: published
// is the manifest's pack.toml for the chapter, dev is the loopback address in
// settings (held to CheckLocalPackURL again here). The one key written is the
// instance's own PreLaunchCommand, by services.SwitchPackSource, which refuses
// a command that is not the launcher's template. The next Play syncs from the
// new pack; packwiz-installer keeps no URL and works from file hashes.
func (a *App) SetPackSource(chapterID, source string) (models.InstanceReport, error) {
	chapter, ok := a.chapter(chapterID)
	if !ok {
		return models.InstanceReport{}, fmt.Errorf("no chapter %q", chapterID)
	}
	if source != packSourcePublished && source != packSourceDev {
		return models.InstanceReport{}, fmt.Errorf("unknown pack source %q", source)
	}
	if err := a.refuseUnderPreview(chapter); err != nil {
		return models.InstanceReport{}, err
	}
	engine := a.realEngine()
	if a.games.Active(chapterID) {
		return models.InstanceReport{}, errGameActive(chapter)
	}
	if !engine.Found {
		return models.InstanceReport{}, services.ErrPrismNotFound
	}
	chapter, cfg, err := a.chapterInstance(chapterID)
	if err != nil {
		return models.InstanceReport{}, err
	}
	to, err := a.packSourceURL(chapter, source)
	if err != nil {
		return models.InstanceReport{}, err
	}
	// A game started from Prism itself is not the tracker's; Prism could be
	// writing the file (ADR-2, second amendment).
	if err := a.refuseIfRunning(chapter, filepath.Dir(cfg)); err != nil {
		return models.InstanceReport{}, err
	}
	result, err := services.SwitchPackSource(cfg, a.syncExe(), to)
	if err != nil {
		slog.Warn("pack source not switched", "chapter", chapterID, "source", source, "error", err)
		return models.InstanceReport{}, fmt.Errorf("%s: %w", chapter.Name, err)
	}
	if result == services.PreLaunchAbsent {
		return models.InstanceReport{}, fmt.Errorf("%s has no pre-launch command to switch", chapter.Name)
	}
	slog.Info("pack source switched", "chapter", chapterID, "source", source, "changed", result == services.PreLaunchRewritten)
	return a.GetInstances()
}

// packSourceURL is the pack.toml a source word stands for: the manifest's, or
// the loopback override in settings. Either way the value is the launcher's,
// never the caller's.
func (a *App) packSourceURL(chapter models.Chapter, source string) (string, error) {
	if source == packSourcePublished {
		if chapter.Pack.Packwiz == nil {
			return "", fmt.Errorf("%s has no published pack yet", chapter.Name)
		}
		return *chapter.Pack.Packwiz, nil
	}
	settings, err := a.settings.Load()
	if err != nil {
		return "", err
	}
	local := strings.TrimSpace(settings.PackOverrides[chapter.ID])
	if local == "" {
		return "", fmt.Errorf("no local pack for %s in settings", chapter.Name)
	}
	if err := services.CheckLocalPackURL(local); err != nil {
		return "", err
	}
	return local, nil
}
