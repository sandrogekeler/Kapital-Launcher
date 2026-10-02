package services

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"kapital/backend/models"
)

// Prism does not exit when it stops a start before the game: with
// ShowConsoleOnError (its default, and the managed root's setting) it opens
// its console window with the error and lives until the player closes it. So
// the process is no signal, and a failed pre-launch sync, a Java download that
// failed or a library that would not fetch would leave the card at "Starting"
// until the start timeout (#103). Prism's own launcher log does say so:
// Task::emitFailed writes a line for each task that failed, and the one to
// act on is the LaunchTask's, which every launch step that stops a start ends
// in. Other task classes (a news or metadata download) fail without stopping a
// launch and are not matched.
//
// Like the game's log, this one carries local paths and may carry names, so
// it is read for its markers only: no line, and no part of one, is logged,
// stored or emitted (docs/adr/0002-prism-data-root.md, third amendment).
//
// The line is derived from Prism 11.1.1's source. The exact quoting of the
// reason is QDebug's and has not been seen on a real file, so the match is on
// substrings and never on a whole line [verify].
const (
	prismTaskCategory = "[launcher.task]"
	prismLaunchTask   = "LaunchTask("
	prismTaskFailed   = "failed:"
	// prismPackSyncReason is the start of the reason when the pre-launch
	// command, which runs the pack sync, exits non-zero, crashes or does not
	// start.
	prismPackSyncReason = "Pre-Launch command failed"
)

// prismLogPath is Prism's launcher log under its data root. Prism truncates it
// at every start and rotates the older ones to -1 .. -4.
func prismLogPath(root string) string {
	return filepath.Join(root, "logs", "PrismLauncher-0.log")
}

// prismTaskFailure reports whether a line of Prism's log is a launch step
// failing, and why, as a GameFail constant.
func prismTaskFailure(line []byte) (string, bool) {
	if !bytes.Contains(line, []byte(prismTaskCategory)) ||
		!bytes.Contains(line, []byte(prismLaunchTask)) ||
		!bytes.Contains(line, []byte(prismTaskFailed)) {
		return "", false
	}
	if bytes.Contains(line, []byte(prismPackSyncReason)) {
		return models.GameFailPackSync, true
	}
	return models.GameFailLaunch, true
}

// PrismLogSnapshot is Prism's launcher log as it was before Play: whether it
// was there, how long and its first line's digest.
type PrismLogSnapshot struct {
	exists bool
	size   int64
	head   logHead
}

// SnapshotPrismLog records Prism's launcher log under root as it is now. Taken
// before Prism is started, so a failure left by an earlier start is not taken
// for this one. An empty root has nothing to record.
func SnapshotPrismLog(root string) PrismLogSnapshot {
	if root == "" {
		return PrismLogSnapshot{}
	}
	path := prismLogPath(root)
	info, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("prism log snapshot", "error", err)
		}
		return PrismLogSnapshot{}
	}
	head, err := readLogHead(path)
	if err != nil {
		slog.Warn("prism log snapshot", "error", err)
		return PrismLogSnapshot{}
	}
	return PrismLogSnapshot{exists: true, size: info.Size(), head: head}
}

// prismLogFollower reads what Prism's launcher log gains while a start waits
// for the game. It begins where the snapshot ended: Prism truncates the log
// when it starts, so a file that is shorter, or whose first line is another,
// is read from its beginning, and one that only grew is a Prism already open
// logging this launch (it is handed the launch over a local socket) and is
// read from the snapshot's end. A log that is missing, truncated or replaced
// between reads is never an error.
type prismLogFollower struct {
	path    string
	head    logHead
	offset  int64
	partial []byte
	// reason is why a launch step failed, "" until one has.
	reason string
	// warned is whether a failed read has been logged: once is enough.
	warned bool
}

// newPrismLogFollower follows the log under root from the snapshot, nil when
// there is no root.
func newPrismLogFollower(root string, before PrismLogSnapshot) *prismLogFollower {
	if root == "" {
		return nil
	}
	f := &prismLogFollower{path: prismLogPath(root)}
	if before.exists {
		f.head, f.offset = before.head, before.size
	}
	return f
}

// poll reads whatever the log gained since the last poll and returns why a
// launch step failed, "" when none has.
func (f *prismLogFollower) poll() string {
	info, err := os.Stat(f.path)
	if err != nil {
		// Not written yet, or gone between polls. One that comes back is told
		// from this one by its length and first line, below.
		f.noteError(err)
		return f.reason
	}
	head, err := readLogHead(f.path)
	if err != nil {
		f.noteError(err)
		return f.reason
	}
	switch {
	case info.Size() < f.offset, f.head.complete && head.complete && head.first != f.head.first:
		f.head, f.offset, f.partial = head, 0, nil
	case head.complete:
		f.head = head
	}
	for f.reason == "" && info.Size() > f.offset {
		n, rest, err := readLines(f.path, f.offset, info.Size()-f.offset, f.partial, f.line)
		if err != nil {
			f.noteError(err)
			break
		}
		if n == 0 {
			break
		}
		f.offset += int64(n)
		f.partial = rest
	}
	return f.reason
}

// line notes a launch step failing. The bytes are not kept.
func (f *prismLogFollower) line(b []byte) {
	if reason, ok := prismTaskFailure(b); ok && f.reason == "" {
		f.reason = reason
	}
}

// noteError logs a read that failed, once and at debug: the run goes on
// without the log. A file that is not there is expected and not logged at all,
// and the error is never the log's content.
func (f *prismLogFollower) noteError(err error) {
	if errors.Is(err, os.ErrNotExist) || f.warned {
		return
	}
	f.warned = true
	slog.Debug("prism log read", "error", err)
}

// prismFailed ends the run when Prism's log says a launch step failed. Only
// while the start waits for the game: once its log is fresh the JVM is up, and
// a failure after that is the game's, which the game's log and process cover.
func (r *gameRun) prismFailed(now time.Time) bool {
	if r.prism == nil || r.state.Phase != models.GamePhaseStarting || r.follower.Fresh() {
		return false
	}
	reason := r.prism.poll()
	if reason == "" {
		return false
	}
	slog.Info("prism stopped the start", "chapter", r.req.ChapterID, "reason", reason)
	r.state.Reason = reason
	r.set(models.GamePhaseFailed, now, nil)
	return true
}
