package main

import (
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
