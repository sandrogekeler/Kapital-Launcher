package main

import (
	"errors"
	"fmt"
	"log/slog"

	"kapital/backend/models"
)

// GetPlayerProfile looks the profile name up on Mojang's public endpoints (issue
// 193, ADR-13): the UUID, Mojang's own spelling and the face cut from the skin,
// cached for a day. The name is checked against Minecraft's own rule before any
// request, and only it leaves the machine. Not finding the name, being offline
// or an invalid name are statuses of the result, never an error: the page falls
// back to the initials and says nothing about it.
func (a *App) GetPlayerProfile(name string) (models.PlayerProfile, error) {
	profile := a.mojang.Profile(a.context(), name)
	slog.Info("player profile", "status", profile.Status)
	return profile, nil
}

// CopyPlayerUUID puts the UUID of the profile the latest GetPlayerProfile found
// on the clipboard. It takes no argument from the page: the value is the one Go
// holds, so the clipboard only ever gets a UUID Mojang returned.
func (a *App) CopyPlayerUUID() error {
	uuid, err := a.mojang.CopyUUID()
	if err != nil {
		return err
	}
	if err := a.setClipboard(a.context(), uuid); err != nil {
		slog.Error("copy player uuid", "error", err)
		return fmt.Errorf("copy UUID: %w", err)
	}
	return nil
}

// learnPlayerName keeps the name the game signed in with on Prism's default
// account, so the account page can show it and its face without the player
// typing it (the launcher never reads Prism's accounts). It is kept only
// while the settings still launch that way: no profile name, Play offline
// off. The log says the outcome and never the name.
func (a *App) learnPlayerName(name string) {
	learned := false
	err := a.settings.Update(func(s *models.AppSettings) error {
		if s.ProfileName != "" || s.Offline || s.PlayerName == name {
			return errUnchanged
		}
		s.PlayerName, learned = name, true
		return nil
	})
	switch {
	case errors.Is(err, errUnchanged):
	case err != nil:
		slog.Warn("learn player name", "error", err)
	case learned:
		slog.Info("learned player name")
	}
}

// errUnchanged stops a settings update that has nothing to write.
var errUnchanged = errors.New("unchanged")
