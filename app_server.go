package main

import (
	"log/slog"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"

	"kapital/backend/models"
	"kapital/backend/services"
)

// serverAddress is the address a chapter's server is reached at now: its saved
// choice, else its first address (services.ServerAddress). The status ticker
// asks it on every tick, and the pings, Play's --server and the redactor's
// list all come from the same resolver. Settings that cannot be read are the
// defaults, so the first address.
func (a *App) serverAddress(chapter models.Chapter) string {
	settings, err := a.settings.Load()
	if err != nil {
		slog.Warn("settings for the server choice", "error", err)
	}
	return services.ServerAddress(chapter, settings.ServerChoices)
}

// joinAddress is what Play passes to Prism's --server: the player's chosen
// address of the chapter's list when the player turned the chapter's join
// switch on (issue 163), else "": off for every chapter until they do.
// LaunchArgs still validates it.
func joinAddress(chapter models.Chapter, settings models.AppSettings) string {
	if chapter.Server == nil || !services.JoinsServer(settings, chapter.ID) {
		return ""
	}
	return services.ServerAddress(chapter, settings.ServerChoices)
}

// checkServer pings a chapter's chosen address now, stores the result and
// emits it as a server:status event, so the line follows with no polling. An
// answer from an address the player has moved away from while it was out is
// not emitted: the save that moved it pings the new one.
func (a *App) checkServer(chapter models.Chapter) models.ServerStatus {
	dialed := a.serverAddress(chapter)
	status := a.status.Check(a.context(), chapter, dialed)
	if a.ctx != nil && a.serverAddress(chapter) == dialed {
		wailsrt.EventsEmit(a.ctx, services.EventServerStatus, status)
	}
	return status
}

// changedServers are the chapters whose server address a save moves: the
// resolved address before and after differ. A choice written down that names
// the default anyway moves nothing.
func (a *App) changedServers(before, after models.AppSettings) []models.Chapter {
	var changed []models.Chapter
	for _, c := range a.manifest.Chapters {
		if services.ServerAddress(c, before.ServerChoices) != services.ServerAddress(c, after.ServerChoices) {
			changed = append(changed, c)
		}
	}
	return changed
}

// recheckServers pings, in the background, every chapter a save moved to
// another address, so its status event follows the choice at once instead of
// at the next tick. The save does not wait on a dial.
func (a *App) recheckServers(before, after models.AppSettings) {
	for _, c := range a.changedServers(before, after) {
		go a.checkServer(c)
	}
}
