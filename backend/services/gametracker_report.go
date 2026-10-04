package services

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"kapital/backend/models"
)

// The run report (ADR-2, sixth amendment) is what the launcher shows in place
// of Prism's console when a start fails or the game crashes: the run's phases,
// the redacted end of the game's log and the name of its crash report. It is
// built from the disk when asked for, for the card when a run ends badly and
// for the player opening it, and nothing read is kept: the tracker holds where
// the log is and when each phase was reached, which is not the log.

// ErrNoRun is a report asked for a chapter this app run never launched.
var ErrNoRun = errors.New("there is no run to report on")

// playerHeadBytes bounds how much of the log's start is read for the player's
// name when the tail does not have it. A big pack prints a lot before the
// client says who is playing, so this is generous, and still a bound.
const playerHeadBytes = 8 << 20

// A name is learned from the client's own line, which vanilla and the loaders
// print early, or failing that from the argument list a loader prints first.
var (
	settingUser  = regexp.MustCompile(`Setting user: (\S+)`)
	usernameArg  = regexp.MustCompile(`--username,?\s+([^\s,\]]+)`)
	playerFilter = []byte("Setting user: ")
	argFilter    = []byte("--username")
)

// runRecord is what the tracker keeps of a chapter's latest run for its report:
// where the game's log is, once found, and when each phase was reached. Times
// and a path, none of the log's content.
type runRecord struct {
	startedAt time.Time
	// logPath is the game log of this run, "" until a fresh one was found: a
	// start that failed before the game has none.
	logPath string
	// reachedMs maps each timed phase the log showed to the milliseconds from
	// Play, as LaunchTiming does.
	reachedMs map[string]int64
}

// record is the run as the report needs it. The run's own goroutine calls it,
// with the tracker's lock held, wherever it moves the run on.
func (r *gameRun) record() runRecord {
	return runRecord{startedAt: r.req.StartedAt, logPath: r.follower.path, reachedMs: r.timing().PhaseMs}
}

// UseRedactor sets how a report makes its redactor. The app builds one from
// the settings as they are at the time, so a profile name saved after launch is
// still masked. Without one a report is refused, never made unredacted.
func (t *GameTracker) UseRedactor(build func() (*Redactor, error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.redactor = build
}

// Report is what the launcher knows of the chapter's latest run, now or ended.
// It reads the game's log tail and the crash reports folder's listing, only
// here and never while the run goes on, and keeps neither. A run with no game
// log of its own (the start failed before the game) has the phases and no log
// or crash report: an earlier run's must not be shown as this one's.
func (t *GameTracker) Report(chapterID string) (models.RunReport, error) {
	t.mu.Lock()
	state, ok := t.states[chapterID]
	rec := t.records[chapterID]
	build := t.redactor
	t.mu.Unlock()
	if !ok {
		return models.RunReport{}, ErrNoRun
	}
	report := models.RunReport{
		Game: state, Phases: phaseTimes(rec.reachedMs),
		ConsoleAvailable: t.consoleAvailable(chapterID),
	}
	if rec.logPath == "" {
		return report, nil
	}
	if build == nil {
		return models.RunReport{}, errors.New("no redactor to read the game log with")
	}
	redactor, err := build()
	if err != nil {
		return models.RunReport{}, err
	}
	if err := fillLog(&report, rec.logPath, redactor); err != nil {
		return models.RunReport{}, err
	}
	// The game folder is two up from logs/latest.log.
	report.CrashReport = newestCrashReport(filepath.Dir(filepath.Dir(rec.logPath)), rec.startedAt)
	slog.Info("run report", "chapter", chapterID, "phase", state.Phase, "logLines", report.LogLines, "crashReport", report.CrashReport != "")
	return report, nil
}

// phaseTimes puts "starting" at 0 and then each timed phase the log showed, in
// the order the game goes through them.
func phaseTimes(reachedMs map[string]int64) []models.PhaseTime {
	out := []models.PhaseTime{{Phase: models.GamePhaseStarting}}
	for _, phase := range []string{models.GamePhaseMods, models.GamePhaseWindow, models.GamePhaseResources, models.GamePhaseRunning} {
		if ms, ok := reachedMs[phase]; ok {
			out = append(out, models.PhaseTime{Phase: phase, Ms: ms})
		}
	}
	return out
}

// fillLog sets the report's log tail: the end of the log, whole lines, with the
// player's in-game name learned from the file masked along with what r masks.
func fillLog(report *models.RunReport, path string, r *Redactor) error {
	raw, err := readLogTail(path, models.RunReportLogBytes)
	if err != nil {
		if errors.Is(err, ErrLogEmpty) {
			return nil
		}
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	if info, err := os.Stat(path); err == nil {
		report.LogTruncated = info.Size() > models.RunReportLogBytes
	}
	name := playerName(raw)
	if name == "" && report.LogTruncated {
		name = playerNameInHead(path)
	}
	text := r.WithPlayer(name).Redact(string(raw))
	report.LogTail = text
	report.LogLines = strings.Count(text, "\n")
	return nil
}

// playerName is the in-game name a stretch of log gives, "" when it gives none.
func playerName(text []byte) string {
	for _, re := range []*regexp.Regexp{settingUser, usernameArg} {
		if m := re.FindSubmatch(text); m != nil {
			return string(m[1])
		}
	}
	return ""
}

// playerNameInHead reads the log from its start, up to playerHeadBytes, for the
// line that gives the player's name. A line is looked at and dropped.
func playerNameInHead(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close() //nolint:errcheck // read-only handle, nothing to flush
	return playerNameInStream(f)
}

// playerNameInStream is playerNameInHead over any reader of a log's text, up to
// playerHeadBytes of it.
func playerNameInStream(r io.Reader) string {
	scan := bufio.NewScanner(io.LimitReader(r, playerHeadBytes))
	// A line longer than this ends the search: nothing worth reading is that
	// long.
	scan.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for scan.Scan() {
		line := scan.Bytes()
		if bytes.Contains(line, playerFilter) || bytes.Contains(line, argFilter) {
			if name := playerName(line); name != "" {
				return name
			}
		}
	}
	return ""
}

// newestCrashReport is the name of the newest file in the game folder's
// crash-reports written since Play, "" when there is none or no folder. The
// name only: the file is never opened, and the folder button reaches it.
func newestCrashReport(gameDir string, since time.Time) string {
	entries, err := os.ReadDir(filepath.Join(gameDir, "crash-reports"))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("crash reports folder", "error", err)
		}
		return ""
	}
	var newest string
	var at time.Time
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() || info.ModTime().Before(since) {
			continue
		}
		if newest == "" || info.ModTime().After(at) {
			newest, at = e.Name(), info.ModTime()
		}
	}
	return newest
}
