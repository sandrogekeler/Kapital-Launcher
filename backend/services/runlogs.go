package services

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"kapital/backend/models"
)

// The logs page (issue 155) lists an instance's game logs and crash reports and
// reads one of them on the player's request. It is read only: nothing is
// written, moved or deleted, and nothing read is kept. Both calls take a game
// folder the App resolved from a chapter id, and a file is named by its kind and
// base name, which must be one the listing would produce.
//
// Every file is reached through an *os.Root on the game folder, as the zip
// extractor does (SECURITY_CHECKLIST S4.5): the Root refuses a path that leaves
// the folder, a link included, so a symlink in logs/ or crash-reports/ cannot
// lead a read out.

const (
	// RunLogsPerKind caps each kind's list: the newest of the logs and the newest
	// of the crash reports.
	RunLogsPerKind = 50
	// maxUnpackedRunLog is the most a dated log may unpack to. A gzip can be a
	// thousand times its size, so the stream is counted and refused past it. A
	// heavy pack's day of debug output is a few tens of megabytes.
	maxUnpackedRunLog = 256 << 20
	// crashSlack is how long after a log's last write a crash report may still
	// have been written in its run: the crash is reported before the log ends.
	crashSlack = 2 * time.Minute
	// crashOldestWindow is how far back the oldest log's run is taken to reach,
	// when there is no older log to say when it began.
	crashOldestWindow = 24 * time.Hour
)

// ErrRunLogName is returned for a kind or a name the listing would not produce,
// and for one that is not (or no longer) a regular file in its folder.
var ErrRunLogName = errors.New("not a log or crash report of this instance")

// ErrRunLogTooLarge is returned for a dated log that unpacks to more than the cap.
var ErrRunLogTooLarge = errors.New("the log is too large to unpack")

// plainRunLogName is the shape of a base name the page handles: plain
// characters, no separator, no drive, no stream marker, no leading dot.
var plainRunLogName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,119}$`)

// runLogFolder is the folder of the game folder that holds a kind of file, ""
// for a kind the page does not have.
func runLogFolder(kind string) string {
	switch kind {
	case models.RunLogKindLog:
		return "logs"
	case models.RunLogKindCrash:
		return "crash-reports"
	}
	return ""
}

// datedRunLogName is the game's archive of one run's latest.log: the date and
// the run's number that day, as log4j names it.
var datedRunLogName = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-\d+\.log\.gz$`)

// validRunLogName is the one test of a name, used by the listing to choose what
// to show and by the read to refuse what is not shown: a plain base name that
// is latest.log or a dated archive of it for a log, and a *.txt for a crash
// report. NeoForge also archives a debug-N.log.gz beside every run (seen on
// the author's Frangfurd instance, 2026-10-04): it is the same run at debug
// level, so it is left out, or every run would show twice and the crash mark's
// window, which runs from the previous archive, would shrink to a second.
func validRunLogName(kind, name string) bool {
	if !plainRunLogName.MatchString(name) || strings.Contains(name, "..") {
		return false
	}
	switch kind {
	case models.RunLogKindLog:
		return name == "latest.log" || datedRunLogName.MatchString(name)
	case models.RunLogKindCrash:
		return strings.HasSuffix(name, ".txt")
	}
	return false
}

// GameFolder is gameFolder for the App: the instance's game folder, or "".
func GameFolder(instanceDir string) string { return gameFolder(instanceDir) }

// gameFolder is the instance's game folder as Prism names it, "minecraft" or,
// in older instances, ".minecraft", "" when it has neither.
func gameFolder(instanceDir string) string {
	if instanceDir == "" {
		return ""
	}
	for _, game := range []string{"minecraft", ".minecraft"} {
		dir := filepath.Join(instanceDir, game)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return ""
}

// runFile is a listed file before it becomes a models.RunLog.
type runFile struct {
	kind    string
	name    string
	at      time.Time
	size    int64
	crashed bool
}

// ListRunLogs lists the instance's logs/latest.log, logs/*.log.gz and
// crash-reports/*.txt, newest first, at most RunLogsPerKind of each kind. An
// instance with no game folder, or no such folder in it, has none.
//
// A log is marked Crashed when a crash report was written during its run. The
// game archives a run's log to a dated .log.gz when the next run starts, so a
// dated log's modified time is when the next run began, and a run lasts from the
// next older log's modified time to its own (to its last write plus crashSlack
// for latest.log). A crash report whose modified time is inside that window
// belongs to the run. The oldest log has no older one to start from and is
// taken to have begun crashOldestWindow before its end. It is a rule over file
// times and never reads a file: best effort, wrong when files are copied and
// their times reset, or when a run spans a midnight rollover (the report is
// then marked on the log that continues after it).
func ListRunLogs(instanceDir string) ([]models.RunLog, error) {
	out := []models.RunLog{}
	game := gameFolder(instanceDir)
	if game == "" {
		return out, nil
	}
	root, err := os.OpenRoot(game)
	if err != nil {
		return nil, fmt.Errorf("open game folder: %w", err)
	}
	defer closeReadOnly(root)
	logs, err := listRunFiles(root, models.RunLogKindLog)
	if err != nil {
		return nil, err
	}
	crashes, err := listRunFiles(root, models.RunLogKindCrash)
	if err != nil {
		return nil, err
	}
	markCrashed(logs, crashes)
	all := slices.Concat(logs[:min(len(logs), RunLogsPerKind)], crashes[:min(len(crashes), RunLogsPerKind)])
	sort.SliceStable(all, func(i, j int) bool { return newerFirst(all[i], all[j]) })
	for _, f := range all {
		out = append(out, models.RunLog{
			Kind: f.kind, Name: f.name, ModifiedAt: f.at.UTC().Format(time.RFC3339),
			Size: f.size, Crashed: f.crashed,
		})
	}
	return out, nil
}

// newerFirst orders files by modified time, newest first, then by name so the
// order never depends on the directory's.
func newerFirst(a, b runFile) bool {
	if !a.at.Equal(b.at) {
		return a.at.After(b.at)
	}
	return a.name > b.name
}

// listRunFiles is the regular files of a kind's folder whose names the page
// handles, newest first. A folder that is not there lists nothing.
func listRunFiles(root *os.Root, kind string) ([]runFile, error) {
	dirName := runLogFolder(kind)
	dir, err := root.Open(dirName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open %s: %w", dirName, err)
	}
	defer closeReadOnly(dir)
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", dirName, err)
	}
	var files []runFile
	for _, e := range entries {
		if !validRunLogName(kind, e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, runFile{kind: kind, name: e.Name(), at: info.ModTime(), size: info.Size()})
	}
	sort.Slice(files, func(i, j int) bool { return newerFirst(files[i], files[j]) })
	return files, nil
}

// markCrashed sets crashed on each of logs (newest first) a crash report's time
// falls in the run of, as ListRunLogs explains.
func markCrashed(logs, crashes []runFile) {
	for i := range logs {
		from := logs[i].at.Add(-crashOldestWindow)
		if i+1 < len(logs) {
			from = logs[i+1].at
		}
		to := logs[i].at.Add(crashSlack)
		for _, c := range crashes {
			if c.at.After(from) && !c.at.After(to) {
				logs[i].crashed = true
				break
			}
		}
	}
}

// ReadRunLog returns one chunk of a listed file, redacted by r: the end of the
// file when before is 0 or past its end, otherwise the stretch that ends at the
// byte offset before (the Offset of the chunk after it). A chunk is at most
// models.RunLogChunkBytes and starts on a line. A dated log is unpacked, and
// refused past maxUnpackedRunLog. The in-game name is learned from the chunk,
// else from the file's head, else from latest.log's, and masked with the rest.
func ReadRunLog(instanceDir, kind, name string, before int64, r *Redactor) (models.RunLogText, error) {
	return readRunLog(instanceDir, kind, name, before, r, maxUnpackedRunLog)
}

func readRunLog(instanceDir, kind, name string, before int64, r *Redactor, unpackMax int64) (models.RunLogText, error) {
	got, err := readRunChunk(instanceDir, kind, name, before, r, unpackMax)
	return got.text, err
}

// runChunk is a chunk as readRunChunk read it: the text for the page, and what
// the live log needs to carry on from it.
type runChunk struct {
	text models.RunLogText
	// end is the byte offset in the file just after the last byte of the chunk
	// that was kept: for the end of latest.log, the line after which a follower
	// reads on.
	end int64
	// player is the in-game name the chunk was masked with, "" when none.
	player string
}

// errRunLogMissing is ErrRunLogName for a file, or a game folder, that is not
// there (yet): the live log waits for latest.log, where a read refuses it.
var errRunLogMissing = fmt.Errorf("%w: it is not there", ErrRunLogName)

func readRunChunk(instanceDir, kind, name string, before int64, r *Redactor, unpackMax int64) (runChunk, error) {
	dirName := runLogFolder(kind)
	if dirName == "" || !validRunLogName(kind, name) {
		return runChunk{}, ErrRunLogName
	}
	if r == nil {
		return runChunk{}, errors.New("no redactor to read the log with")
	}
	game := gameFolder(instanceDir)
	if game == "" {
		return runChunk{}, errRunLogMissing
	}
	root, err := os.OpenRoot(game)
	if err != nil {
		return runChunk{}, fmt.Errorf("open game folder: %w", err)
	}
	defer closeReadOnly(root)
	rel := filepath.Join(dirName, name)
	// Lstat: a link is refused whatever it points at, as the listing skips it.
	info, err := root.Lstat(rel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return runChunk{}, errRunLogMissing
		}
		return runChunk{}, fmt.Errorf("read %s: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		return runChunk{}, ErrRunLogName
	}
	gz := strings.HasSuffix(name, ".gz")
	size := info.Size()
	if gz {
		if size, err = unpackedSize(root, rel, unpackMax); err != nil {
			return runChunk{}, err
		}
	}
	end := size
	if before > 0 && before < size {
		end = before
	}
	start := max(end-models.RunLogChunkBytes, 0)
	// One byte before the chunk says whether it opens on a line start.
	from := max(start-1, 0)
	raw, err := readRunRange(root, rel, gz, from, end-from)
	if err != nil {
		return runChunk{}, fmt.Errorf("read %s: %w", name, err)
	}
	data, offset := wholeLines(raw, from, start)
	// The game writes latest.log while this reads: its last line may be half
	// written, and half a home path would slip past a redactor that matches
	// whole values.
	if end == size && name == "latest.log" {
		data = data[:bytes.LastIndexByte(data, '\n')+1]
	}
	player := learnPlayer(root, rel, gz, data, offset)
	text := r.WithPlayer(player).Redact(strings.ToValidUTF8(string(data), "�"))
	return runChunk{
		text: models.RunLogText{
			Kind: kind, Name: name, Text: text, Offset: offset, Size: size,
			Lines: countLines(text), Truncated: offset > 0,
		},
		end:    offset + int64(len(data)),
		player: player,
	}, nil
}

// wholeLines trims raw, which holds the file from byte from, to start on a line
// at or after start, and says where that is. A chunk with no line start in it
// (one line longer than the chunk) is kept from start as it stands.
func wholeLines(raw []byte, from, start int64) ([]byte, int64) {
	if start == 0 {
		return raw, 0
	}
	if len(raw) == 0 {
		return nil, start
	}
	if i := bytes.IndexByte(raw, '\n'); i >= 0 && i+1 < len(raw) {
		return raw[i+1:], from + int64(i) + 1
	}
	return raw[1:], start
}

// unpackedSize is the length of a gzip file's content, counted without keeping
// it, and refused past max.
func unpackedSize(root *os.Root, rel string, limit int64) (int64, error) {
	rd, closeAll, err := openRunStream(root, rel, true)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", filepath.Base(rel), err)
	}
	defer closeAll()
	n, err := io.Copy(io.Discard, io.LimitReader(rd, limit+1))
	if err != nil {
		return 0, fmt.Errorf("unpack %s: %w", filepath.Base(rel), err)
	}
	if n > limit {
		return 0, ErrRunLogTooLarge
	}
	return n, nil
}

// openRunStream opens a file as a stream of its text, unpacked for a gzip, and
// the func that closes whatever it opened.
func openRunStream(root *os.Root, rel string, gz bool) (io.Reader, func(), error) {
	f, err := root.Open(rel)
	if err != nil {
		return nil, nil, err
	}
	if !gz {
		return f, func() { closeReadOnly(f) }, nil
	}
	zr, err := gzip.NewReader(f)
	if err != nil {
		closeReadOnly(f)
		return nil, nil, err
	}
	return zr, func() { closeReadOnly(zr); closeReadOnly(f) }, nil
}

// readRunRange reads n bytes of a file's text from byte from. A file that is
// shorter than that (it was replaced since it was measured) yields what is there.
func readRunRange(root *os.Root, rel string, gz bool, from, n int64) ([]byte, error) {
	rd, closeAll, err := openRunStream(root, rel, gz)
	if err != nil {
		return nil, err
	}
	defer closeAll()
	if seeker, ok := rd.(io.Seeker); ok {
		if _, err := seeker.Seek(from, io.SeekStart); err != nil {
			return nil, err
		}
	} else if _, err := io.CopyN(io.Discard, rd, from); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(rd, n))
}

// learnPlayer is the in-game name for the redactor: the chunk's own, else the
// file's head when the chunk is not it, else the head of latest.log, which
// names the player of the run it records and so, for a crash report, of the
// same player's last one. "" when none is found.
func learnPlayer(root *os.Root, rel string, gz bool, data []byte, offset int64) string {
	if name := playerName(data); name != "" {
		return name
	}
	if offset > 0 {
		if name := headPlayer(root, rel, gz); name != "" {
			return name
		}
	}
	latest := filepath.Join("logs", "latest.log")
	if rel == latest {
		return ""
	}
	return headPlayer(root, latest, false)
}

// headPlayer is the player named in the first stretch of a file's text.
func headPlayer(root *os.Root, rel string, gz bool) string {
	rd, closeAll, err := openRunStream(root, rel, gz)
	if err != nil {
		return ""
	}
	defer closeAll()
	return playerNameInStream(rd)
}

// countLines counts the lines of a text: a last one without its newline counts.
func countLines(text string) int {
	n := strings.Count(text, "\n")
	if text != "" && !strings.HasSuffix(text, "\n") {
		n++
	}
	return n
}

// closeReadOnly closes a handle opened for reading, which has nothing to flush.
func closeReadOnly(c io.Closer) {
	c.Close() //nolint:errcheck // read-only handle, nothing to flush
}
