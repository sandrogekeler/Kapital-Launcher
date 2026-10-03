package main

import "fmt"

// ShowPrismConsole shows the console of the Prism the launcher started for the
// chapter, which the launcher hid when a start failed (ADR-0012, amendment),
// and gives it the foreground. It returns false when there is none to show: the
// run never failed that way, the loading splash was off, the Prism has gone or
// this is not Windows. The chapter id is looked up in the validated manifest
// like every other, and the window is one the launcher's own hold kept, so
// nothing the caller sends names a window or a process.
func (a *App) ShowPrismConsole(chapterID string) (bool, error) {
	chapter, ok := a.chapter(chapterID)
	if !ok {
		return false, fmt.Errorf("no chapter %q", chapterID)
	}
	// A previewed run (#124) has no console: say so, and touch no window.
	if a.previews.Chapter(chapter.ID) != "" {
		return false, nil
	}
	return a.games.ShowConsole(chapter.ID)
}
