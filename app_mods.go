package main

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"kapital/backend/models"
	"kapital/backend/services"
)

// chapterGameDir is the instance's game folder, the one that holds mods, or ""
// before the pack's first sync made it.
func chapterGameDir(cfg string) string {
	return services.GameFolder(filepath.Dir(cfg))
}

// GetChapterMods lists a chapter's mods for the settings page (issue 156): every
// jar in the instance's mods folder with whether it is switched off, the
// manifest's quick toggles resolved against them, and whether the game looks to
// be running. The chapter id resolves through the manifest and the folder is the
// one the instance report gives, so nothing the caller sends names a path.
//
// When the game is not running it first puts away what a sync run that was
// killed (Prism's cancel is a hard kill) left enabled, from its journal and
// only for mods still on the player's list, so the page shows what the player
// chose and not a half-finished run.
func (a *App) GetChapterMods(chapterID string) (models.ChapterMods, error) {
	chapter, cfg, err := a.chapterInstance(chapterID)
	if err != nil {
		return models.ChapterMods{}, err
	}
	return a.chapterMods(chapter, cfg, true)
}

// chapterMods builds the view. recoverJournal puts a killed run's mods away first,
// which only ever happens while the game is not running.
func (a *App) chapterMods(chapter models.Chapter, cfg string, recoverJournal bool) (models.ChapterMods, error) {
	tracker, guess := a.chapterRunning(chapter, filepath.Dir(cfg))
	view := models.ChapterMods{
		ChapterID: chapter.ID,
		Mods:      []models.ModFile{},
		Toggles:   services.ModToggleStates(chapter.Pack.Toggles, nil),
		Running:   tracker || guess,
	}
	gameDir := chapterGameDir(cfg)
	if gameDir == "" {
		return view, nil
	}
	if recoverJournal && !view.Running {
		settings, err := a.settings.Load()
		if err != nil {
			return models.ChapterMods{}, err
		}
		if n, err := services.RecoverDisabledMods(gameDir, settings.DisabledMods[chapter.ID]); err != nil {
			slog.Warn("mods journal not recovered", "chapter", chapter.ID, "error", err)
		} else if n > 0 {
			slog.Info("mods put away after an interrupted sync", "chapter", chapter.ID, "mods", n)
		}
	}
	mods, err := services.ListMods(gameDir)
	if err != nil {
		slog.Error("list mods", "chapter", chapter.ID, "error", err)
		return models.ChapterMods{}, a.maskedError(chapter, err)
	}
	view.Mods = mods
	view.Toggles = services.ModToggleStates(chapter.Pack.Toggles, mods)
	return view, nil
}

// SetModsDisabled saves which of a chapter's mods are switched off and applies
// it to the mods folder at once, so the folder matches before the next Play does
// anything. disabled is the whole list: every jar base name named is disabled,
// every other jar of the folder is enabled. Each name must be a plain jar name
// that is in the instance's mods folder now (services.ValidateDisabledMods), the
// chapter resolves through the manifest, a game that runs, or a developer
// preview of the chapter, refuses it, and the tracker's word and the game log's
// are both asked (refuseIfRunning).
//
// The list is the launcher's own, in its settings: the pre-launch sync that
// Prism runs reads it, puts these jars back for packwiz-installer so it
// downloads nothing, and puts them away again (ADR-2, ninth amendment). It is
// saved before the folder is changed, so a rename that fails now is made by the
// next Play. The page's answer is the folder read again.
func (a *App) SetModsDisabled(chapterID string, disabled []string) (models.ChapterMods, error) {
	chapter, ok := a.chapter(chapterID)
	if !ok {
		return models.ChapterMods{}, fmt.Errorf("no chapter %q", chapterID)
	}
	if err := a.refuseUnderPreview(chapter); err != nil {
		return models.ChapterMods{}, err
	}
	chapter, cfg, err := a.chapterInstance(chapterID)
	if err != nil {
		return models.ChapterMods{}, err
	}
	if err := a.refuseIfRunning(chapter, filepath.Dir(cfg)); err != nil {
		return models.ChapterMods{}, err
	}
	gameDir := chapterGameDir(cfg)
	if gameDir == "" {
		return models.ChapterMods{}, fmt.Errorf("%s has no mods yet: Play once so the pack is downloaded", chapter.Name)
	}
	// What a killed sync left is put away by the list that was kept, before the
	// list changes.
	before, err := a.settings.Load()
	if err != nil {
		return models.ChapterMods{}, err
	}
	if _, err := services.RecoverDisabledMods(gameDir, before.DisabledMods[chapter.ID]); err != nil {
		slog.Warn("mods journal not recovered", "chapter", chapter.ID, "error", err)
	}
	clean, err := services.ValidateDisabledMods(gameDir, disabled)
	if err != nil {
		return models.ChapterMods{}, fmt.Errorf("%s: %w", chapter.Name, err)
	}
	if err := a.settings.Update(func(s *models.AppSettings) error {
		next := make(map[string][]string, len(s.DisabledMods)+1)
		for id, names := range s.DisabledMods {
			next[id] = names
		}
		if len(clean) == 0 {
			delete(next, chapter.ID)
		} else {
			next[chapter.ID] = clean
		}
		s.DisabledMods = next
		return nil
	}); err != nil {
		return models.ChapterMods{}, err
	}
	changed, applyErr := services.ApplyDisabledMods(gameDir, clean)
	slog.Info("mods disabled", "chapter", chapter.ID, "disabled", len(clean), "renamed", changed)
	if applyErr != nil {
		slog.Warn("mods not all renamed", "chapter", chapter.ID, "error", applyErr)
		return models.ChapterMods{}, fmt.Errorf("%s: the choice is saved, but not every mod could be renamed now (the next Play does it)", chapter.Name)
	}
	return a.chapterMods(chapter, cfg, false)
}
