package main

import (
	"fmt"

	"kapital/backend/models"
	"kapital/backend/services"
)

// checkMap is the reachability check, a variable so the tests can see which
// address the bound method hands it without a network.
var checkMap = services.CheckMap

// CheckChapterMap says whether the chapter's web map answers (issue 161): one
// short GET of the address the manifest names for the chapter, no redirect
// followed, the body dropped after a few KiB. The map page asks before it
// shows the frame, because an iframe cannot tell an unreachable server. The
// chapter id resolves through the validated manifest and the address is the
// manifest's, never one the caller sends; a chapter with no map is not
// reachable, with an empty address. A failure to reach is an answer, not an
// error: the error is only an unknown chapter.
func (a *App) CheckChapterMap(chapterID string) (models.MapStatus, error) {
	chapter, ok := a.chapter(chapterID)
	if !ok {
		return models.MapStatus{}, fmt.Errorf("no chapter %q", chapterID)
	}
	if chapter.Map == nil {
		return models.MapStatus{Reason: "no map"}, nil
	}
	return checkMap(a.context(), *chapter.Map), nil
}
