package services

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

const (
	// logPollInterval is how often the game's log is looked at while a start
	// is followed.
	logPollInterval = 500 * time.Millisecond
	// firstLineMax bounds how much of a log's head is read to tell one run
	// from another. ModLauncher's first line carries the whole argument list.
	firstLineMax = 8 << 10
	// readChunkMax bounds one read, so a log a modpack has filled with debug
	// output is taken a chunk at a time rather than held whole.
	readChunkMax = 4 << 20
	// partialMax bounds a line with no newline yet. Past it the line is read
	// as it stands and the rest of it is taken for a line of its own.
	partialMax = 1 << 20
)

// logFiles are the places Prism keeps an instance's game log: "minecraft",
// or ".minecraft" in older instances, as InstanceRunning reads them.
func logFiles(instanceDir string) []string {
	if instanceDir == "" {
		return nil
	}
	var out []string
	for _, game := range []string{"minecraft", ".minecraft"} {
		out = append(out, filepath.Join(instanceDir, game, "logs", "latest.log"))
	}
	return out
}

// logHead is a log's first line, as a digest: the launcher compares one run's
// first line with another's and never keeps the line itself.
type logHead struct {
	// complete is whether the line's newline has been written yet. Until it
	// has, the digest is of a line still growing and means nothing.
	complete bool
	first    [sha256.Size]byte
}

// GameLogSnapshot is what an instance's game log looked like before Play: for
// each file that existed, its first line's digest.
type GameLogSnapshot struct {
	files map[string]logHead
}

// SnapshotGameLog records the instance's game log as it is now. Taken before
// Prism is started, so the log of an earlier run is never taken for the new
// one.
func SnapshotGameLog(instanceDir string) GameLogSnapshot {
	snap := GameLogSnapshot{files: map[string]logHead{}}
	for _, path := range logFiles(instanceDir) {
		head, err := readLogHead(path)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				slog.Warn("game log snapshot", "error", err)
			}
			continue
		}
		snap.files[path] = head
	}
	return snap
}

// readLogHead digests a log's first line, reading no more than firstLineMax
// bytes of it.
func readLogHead(path string) (logHead, error) {
	f, err := os.Open(path)
	if err != nil {
		return logHead{}, err
	}
	defer f.Close() //nolint:errcheck // read-only file, nothing to flush
	buf := make([]byte, firstLineMax)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return logHead{}, err
	}
	buf = buf[:n]
	head := logHead{complete: n == firstLineMax}
	if i := bytes.IndexByte(buf, '\n'); i >= 0 {
		buf, head.complete = buf[:i], true
	}
	head.first = sha256.Sum256(bytes.TrimRight(buf, "\r"))
	return head, nil
}

// logFollower reads the fresh game log of one launch as it grows and feeds it
// to a parser. It holds a byte offset, a digest and the parser: no line
// outlives the call that reads it, apart from an unfinished last line kept
// until its newline arrives.
type logFollower struct {
	startedAt time.Time
	before    GameLogSnapshot
	paths     []string

	// path is the log being followed, "" until a fresh one is seen.
	path    string
	head    logHead
	offset  int64
	partial []byte
	parser  logParser
	found   []string
	// lastGrowth is when the log was first seen or last grew.
	lastGrowth time.Time
}

func newLogFollower(instanceDir string, before GameLogSnapshot, startedAt time.Time) *logFollower {
	return &logFollower{startedAt: startedAt, before: before, paths: logFiles(instanceDir)}
}

// Fresh is whether a log of this launch has been found.
func (f *logFollower) Fresh() bool { return f.path != "" }

// LastGrowth is when the log last grew, zero before a fresh log.
func (f *logFollower) LastGrowth() time.Time { return f.lastGrowth }

// poll looks at the log once. It returns the phases the log moved through
// since the last poll, in order, and whether the log began again: a new run
// wrote over it, and the phases that follow start from the beginning.
func (f *logFollower) poll(now time.Time) (phases []string, restarted bool) {
	if f.path == "" {
		f.find(now)
		if f.path == "" {
			return nil, false
		}
	}
	info, err := os.Stat(f.path)
	if err != nil {
		// Gone between polls. One that comes back is seen as a new log.
		return nil, false
	}
	head, err := readLogHead(f.path)
	if err != nil {
		slog.Warn("game log read", "error", err)
		return nil, false
	}
	switch {
	case info.Size() < f.offset, f.head.complete && head.complete && head.first != f.head.first:
		f.reset(head)
		restarted = true
	case head.complete:
		f.head = head
	}
	for info.Size() > f.offset {
		got, err := f.readMore(info.Size() - f.offset)
		if err != nil {
			slog.Warn("game log read", "error", err)
			break
		}
		if got == 0 {
			break
		}
		f.lastGrowth = now
	}
	phases, f.found = f.found, nil
	return phases, restarted
}

// find binds the follower to the log of this launch: a file written no
// earlier than Play, whose first line is not the one it had before, or that
// was not there before.
func (f *logFollower) find(now time.Time) {
	for _, path := range f.paths {
		info, err := os.Stat(path)
		if err != nil || info.ModTime().Before(f.startedAt) {
			continue
		}
		head, err := readLogHead(path)
		if err != nil {
			continue
		}
		if was, ok := f.before.files[path]; ok && was.complete && head.complete && was.first == head.first {
			continue
		}
		f.path = path
		f.reset(head)
		f.lastGrowth = now
		return
	}
}

func (f *logFollower) reset(head logHead) {
	f.head = head
	f.offset = 0
	f.partial = nil
	f.parser = logParser{}
	f.found = nil
}

// readMore reads up to readChunkMax bytes from the offset and feeds every
// finished line to the parser. It returns how many bytes it read.
func (f *logFollower) readMore(remaining int64) (int, error) {
	file, err := os.Open(f.path)
	if err != nil {
		return 0, err
	}
	defer file.Close() //nolint:errcheck // read-only file, nothing to flush
	if _, err := file.Seek(f.offset, io.SeekStart); err != nil {
		return 0, err
	}
	buf := make([]byte, min(remaining, readChunkMax))
	n, err := io.ReadFull(file, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return 0, err
	}
	f.offset += int64(n)
	data := append(f.partial, buf[:n]...)
	for {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			break
		}
		f.line(data[:i])
		data = data[i+1:]
	}
	if len(data) > partialMax {
		f.line(data)
		data = nil
	}
	f.partial = append([]byte(nil), data...)
	return n, nil
}

// line feeds one line to the parser and notes the phase it reached. The
// bytes are not kept.
func (f *logFollower) line(b []byte) {
	if phase, moved := f.parser.Line(string(b)); moved {
		f.found = append(f.found, phase)
	}
}
