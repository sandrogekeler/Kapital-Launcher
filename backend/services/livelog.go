package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kapital/backend/models"
)

// The live log (issue 155) follows one chapter's logs/latest.log while the
// logs page shows it: Go looks at the file, reads what was appended and emits
// it, masked, as "log:live" events. The frontend never polls.
//
// It is the logs page's reader (runlogs.go) kept going, with the same rules: it
// follows only the instance's own logs/latest.log (the name goes through
// validRunLogName), opens it through an *os.Root on the game folder, refuses a
// link, redacts every line with the in-game name learned from the log, writes
// nothing and keeps nothing but a byte offset and the first bytes of the file,
// which is how a new run's file is told from the one it replaced. Nothing it
// reads goes to slog.

// EventLiveLog is the Wails event carrying a models.LiveLogEvent. Every
// listener filters on ChapterID.
const EventLiveLog = "log:live"

const (
	// liveLogEvery is how often latest.log is looked at.
	liveLogEvery = 500 * time.Millisecond
	// liveLogHeadBytes is how much of the file's start is kept to tell a new
	// run's file from the one followed. A log4j line starts with its time, so
	// two runs differ well inside it.
	liveLogHeadBytes = 128
	// liveLogPerTick bounds what one look reads, in events' worth, so a game
	// writing debug output as fast as it can does not hold the follower: the
	// rest is read at the next look.
	liveLogPerTick = 16
)

// LiveLog follows the logs/latest.log of one chapter's instance at a time:
// starting a follower stops the one before it, and the app's shutdown, which
// cancels the context it was started under, stops it too.
type LiveLog struct {
	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	chapter string
	// newTick makes the channel that paces the follower and the func that
	// stops it; a test swaps it for a channel it sends on.
	newTick func() (<-chan time.Time, func())
	// afterPoll, when set, is called when a look at the file is over; a test
	// waits on it.
	afterPoll func()
}

// NewLiveLog returns a LiveLog that looks at the file every 500 ms.
func NewLiveLog() *LiveLog {
	return &LiveLog{newTick: func() (<-chan time.Time, func()) {
		t := time.NewTicker(liveLogEvery)
		return t.C, t.Stop
	}}
}

// Start returns the end of the instance's logs/latest.log exactly as ReadRunLog
// reads it, and follows the file from there: each look at it emits what was
// appended as a models.LiveLogEvent for chapterID, and a file that shrank or
// was replaced (a new run started) emits one with Reset set and the new file's
// end. A latest.log that is not there yet gives an empty text and is waited
// for. The follower ends when ctx is cancelled, on Stop, or when another Start
// takes its place.
func (l *LiveLog) Start(ctx context.Context, chapterID, instanceDir string, r *Redactor, emit func(models.LiveLogEvent)) (models.RunLogText, error) {
	return l.start(ctx, chapterID, instanceDir, models.RunLogKindLog, "latest.log", r, emit)
}

func (l *LiveLog) start(ctx context.Context, chapterID, instanceDir, kind, name string, r *Redactor, emit func(models.LiveLogEvent)) (models.RunLogText, error) {
	// The one file it follows, held to the page's own name rules.
	if kind != models.RunLogKindLog || name != "latest.log" || !validRunLogName(kind, name) {
		return models.RunLogText{}, ErrRunLogName
	}
	if r == nil {
		return models.RunLogText{}, errors.New("no redactor to read the log with")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopLocked()

	got, err := readRunChunk(instanceDir, kind, name, 0, r, maxUnpackedRunLog)
	if err != nil && !errors.Is(err, errRunLogMissing) {
		return models.RunLogText{}, err
	}
	if err != nil {
		got.text = models.RunLogText{Kind: kind, Name: name}
	}
	f := &liveFollower{
		chapter: chapterID,
		dir:     instanceDir,
		rel:     filepath.Join(runLogFolder(kind), name),
		r:       r.WithPlayer(got.player),
		offset:  got.end,
		emit:    emit,
		after:   l.afterPoll,
	}
	f.head = f.readHead()
	tick, stopTick := l.newTick()
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	l.cancel, l.done, l.chapter = cancel, done, chapterID
	go func() {
		defer close(done)
		defer stopTick()
		f.run(runCtx, tick)
	}()
	return got.text, nil
}

// Stop ends the follower of chapterID's log, if that is the one running, and
// returns once it has: no event of it comes after. Another chapter's follower
// is left alone, so a late Stop of the chapter before cannot end the new one.
func (l *LiveLog) Stop(chapterID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cancel != nil && l.chapter == chapterID {
		l.stopLocked()
	}
}

// Shutdown ends whichever follower is running.
func (l *LiveLog) Shutdown() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopLocked()
}

func (l *LiveLog) stopLocked() {
	if l.cancel == nil {
		return
	}
	l.cancel()
	<-l.done
	l.cancel, l.done, l.chapter = nil, nil, ""
}

// liveFollower is one follower's state, owned by its goroutine after start.
type liveFollower struct {
	chapter string
	dir     string
	rel     string
	r       *Redactor
	// offset is the byte after the last line sent; the next read starts there.
	offset int64
	// head is the first bytes of the file followed, to tell it from a new one.
	head  []byte
	emit  func(models.LiveLogEvent)
	after func()
}

func (f *liveFollower) run(ctx context.Context, tick <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick:
			f.poll(ctx)
			if f.after != nil {
				f.after()
			}
		}
	}
}

// open opens the game folder and the file through the root, as the page's reads
// do, refusing a link. ok is false when there is nothing to read now.
func (f *liveFollower) open() (root *os.Root, file *os.File, size int64, ok bool) {
	game := gameFolder(f.dir)
	if game == "" {
		return nil, nil, 0, false
	}
	root, err := os.OpenRoot(game)
	if err != nil {
		return nil, nil, 0, false
	}
	info, err := root.Lstat(f.rel)
	if err != nil || !info.Mode().IsRegular() {
		closeReadOnly(root)
		return nil, nil, 0, false
	}
	file, err = root.Open(f.rel)
	if err != nil {
		closeReadOnly(root)
		return nil, nil, 0, false
	}
	return root, file, info.Size(), true
}

// readHead is the first bytes of the file now, for the start.
func (f *liveFollower) readHead() []byte {
	root, file, size, ok := f.open()
	if !ok {
		return nil
	}
	defer closeReadOnly(root)
	defer closeReadOnly(file)
	return readAtMost(file, 0, min(size, liveLogHeadBytes))
}

// readAtMost reads n bytes from off, fewer when the file ends first.
func readAtMost(file *os.File, off, n int64) []byte {
	buf := make([]byte, n)
	got, err := file.ReadAt(buf, off)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil
	}
	return buf[:got]
}

// sameFile is whether the bytes at the file's start are the ones kept: the
// file is the one that was followed. A head that is shorter than the bytes
// kept, or different in them, is another file; a longer one extends them.
func (f *liveFollower) sameFile(head []byte) bool {
	n := min(len(f.head), len(head))
	if len(head) < len(f.head) || !bytes.Equal(f.head[:n], head[:n]) {
		return false
	}
	f.head = head
	return true
}

// poll looks at the file once: a replaced or shorter file is a reset, else what
// was appended is sent, a piece at a time, up to liveLogPerTick pieces.
func (f *liveFollower) poll(ctx context.Context) {
	root, file, size, ok := f.open()
	if !ok {
		return
	}
	defer closeReadOnly(root)
	defer closeReadOnly(file)
	head := readAtMost(file, 0, min(size, liveLogHeadBytes))
	if size < f.offset || !f.sameFile(head) {
		f.reset(file, size, head)
		return
	}
	for range liveLogPerTick {
		if ctx.Err() != nil || f.offset >= size {
			return
		}
		piece := readAtMost(file, f.offset, min(size-f.offset, models.LiveLogEventBytes))
		cut := bytes.LastIndexByte(piece, '\n') + 1
		if cut == 0 {
			// A line still being written waits for its newline, unless it is
			// longer than an event carries: then it goes as it stands.
			if int64(len(piece)) < models.LiveLogEventBytes {
				return
			}
			cut = len(piece)
		}
		f.offset += int64(cut)
		f.send(piece[:cut], false)
	}
}

// reset follows a file that is not the one it was: it sends the end of the new
// file, from a whole line, with Reset set, and carries on after it.
func (f *liveFollower) reset(file *os.File, size int64, head []byte) {
	f.head = head
	start := max(size-models.LiveLogEventBytes, 0)
	piece := readAtMost(file, start, size-start)
	if start > 0 {
		// The first line of a cut is a part of one.
		if i := bytes.IndexByte(piece, '\n'); i >= 0 {
			start += int64(i) + 1
			piece = piece[i+1:]
		}
	}
	cut := bytes.LastIndexByte(piece, '\n') + 1
	f.offset = start + int64(cut)
	f.send(piece[:cut], true)
}

// send redacts lines and emits them. The in-game name is learned from them
// first, as a read of the file does, so the line that gives it is masked too.
func (f *liveFollower) send(lines []byte, reset bool) {
	if name := playerName(lines); name != "" {
		f.r = f.r.WithPlayer(name)
	}
	text := f.r.Redact(strings.ToValidUTF8(string(lines), "�"))
	f.emit(models.LiveLogEvent{ChapterID: f.chapter, Lines: text, Reset: reset})
	if reset {
		slog.Info("live log reset", "chapter", f.chapter)
	}
}
