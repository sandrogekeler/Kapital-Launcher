package services

import (
	"log/slog"
	"path/filepath"
	"strings"
)

// An open Prism overwrites the launcher's writes to instance.cfg (issue 209,
// ADR-2 thirteenth amendment). Prism keeps each instance's settings in memory
// and writes the whole file at a launch, on a settings save and on exit, so a
// key the launcher wrote while a Prism had the instance loaded comes back as it
// was. Before it writes, the launcher closes the Prism it started for the chapter
// (CloseOwnPrism) and refuses while any other Prism of that executable is open
// (OtherPrismOpen). Nothing here ends a process it did not start: the second
// only looks.

// CloseOwnPrism closes the Prism the launcher started for the chapter, when it is
// still alive after its run, with Stop's routine (CloseConsole: asked to close,
// ended by its pid if it is still there, the exit waited for), and says whether
// that Prism is gone. A chapter with no record, or whose Prism has exited, is
// gone. A Prism that is alive and not the launcher's to close, one whose run is
// still going, is not closed and is not gone: the caller refuses.
func (t *GameTracker) CloseOwnPrism(chapterID string) bool {
	c := t.console(chapterID)
	if c == nil || !c.alive() {
		return true
	}
	if !c.leftBehind() {
		return false
	}
	t.CloseConsole(chapterID)
	return !c.alive()
}

// OtherPrismOpen says whether a Prism other than the chapter's own, closed above,
// is running from the executable exe: a process whose name is exe's and whose
// image path is exe's. The processes are listed by pid, parent pid and name, as
// the tracker does for the game, and the image path is read for those whose name
// matches and for no other (ADR-2 thirteenth amendment). A process whose path
// cannot be read counts as open: it has Prism's name, and a refused write is the
// safe side of one that is lost. An empty exe, or an OS with no process lookup,
// cannot tell, and an error says so: the caller goes on as it did before.
func (t *GameTracker) OtherPrismOpen(chapterID, exe string) (bool, error) {
	if exe == "" {
		return false, nil
	}
	procs, err := t.os.list()
	if err != nil {
		return false, err
	}
	// The chapter's own Prism, once it has exited, may still be in the list for
	// a moment: it is not an open one.
	skip := 0
	if c := t.console(chapterID); c != nil && !c.alive() {
		skip = c.pid
	}
	pids := matchPrismProcesses(procs, t.os.image, exe, skip)
	if len(pids) > 0 {
		slog.Info("another prism is open", "chapter", chapterID, "processes", len(pids))
	}
	return len(pids) > 0, nil
}

// matchPrismProcesses is the pids of the processes that are the Prism exe: the
// name is the base name of exe (the one the OS lists, in any case) and the image
// path, where it can be read, is exe itself. image may be nil, and then the name
// alone decides. skip is a pid to leave out.
func matchPrismProcesses(procs []procInfo, image func(pid int) (string, bool), exe string, skip int) []int {
	name := filepath.Base(exe)
	want := normalPath(exe)
	var pids []int
	for _, p := range procs {
		if p.PID == skip || !strings.EqualFold(p.Name, name) {
			continue
		}
		if image != nil {
			if path, ok := image(p.PID); ok && !strings.EqualFold(normalPath(path), want) {
				continue
			}
		}
		pids = append(pids, p.PID)
	}
	return pids
}

// normalPath is a path as two spellings of one file meet: cleaned, with links
// and 8.3 short names resolved where the file can be found.
func normalPath(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	return filepath.Clean(p)
}
