package services

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"kapital/backend/models"
)

var runLogsBase = time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)

// runLogRig is an instance folder with a game folder to write logs into.
type runLogRig struct {
	t        *testing.T
	instance string
	game     string
}

func newRunLogRig(t *testing.T) *runLogRig {
	t.Helper()
	instance := t.TempDir()
	game := filepath.Join(instance, "minecraft")
	for _, dir := range []string{"logs", "crash-reports"} {
		if err := os.MkdirAll(filepath.Join(game, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &runLogRig{t: t, instance: instance, game: game}
}

// write puts a file in the game folder with a modified time.
func (r *runLogRig) write(rel string, body []byte, at time.Time) {
	r.t.Helper()
	path := filepath.Join(r.game, filepath.FromSlash(rel))
	if err := os.WriteFile(path, body, 0o644); err != nil {
		r.t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		r.t.Fatal(err)
	}
}

func gz(t *testing.T, text string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write([]byte(text)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (r *runLogRig) list() []models.RunLog {
	r.t.Helper()
	out, err := ListRunLogs(r.instance)
	if err != nil {
		r.t.Fatal(err)
	}
	return out
}

func (r *runLogRig) read(kind, name string, before int64) (models.RunLogText, error) {
	return ReadRunLog(r.instance, kind, name, before, runLogRedactor())
}

func runLogRedactor() *Redactor {
	return NewRedactor(`C:\Users\sandro`, "", []string{"play.kapitel.example:25565"}, "sandro")
}

func names(logs []models.RunLog) []string {
	var out []string
	for _, l := range logs {
		out = append(out, l.Kind+":"+l.Name)
	}
	return out
}

func TestListRunLogsIsNewestFirstAcrossBothKinds(t *testing.T) {
	r := newRunLogRig(t)
	r.write("logs/latest.log", []byte("now\n"), runLogsBase.Add(3*time.Hour))
	r.write("logs/2026-10-03-1.log.gz", gz(t, "a\n"), runLogsBase.Add(2*time.Hour))
	r.write("logs/2026-10-02-1.log.gz", gz(t, "b\n"), runLogsBase.Add(-20*time.Hour))
	r.write("crash-reports/crash-2026-10-03_12.30.00-client.txt", []byte("boom\n"), runLogsBase.Add(30*time.Minute))
	// Files the page does not list: another extension, a debug log, a folder.
	r.write("logs/debug.log", []byte("x"), runLogsBase)
	r.write("logs/readme.txt", []byte("x"), runLogsBase)
	r.write("crash-reports/notes.md", []byte("x"), runLogsBase)
	if err := os.Mkdir(filepath.Join(r.game, "logs", "sub.log.gz"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := r.list()
	want := []string{
		"log:latest.log",
		"log:2026-10-03-1.log.gz",
		"crash:crash-2026-10-03_12.30.00-client.txt",
		"log:2026-10-02-1.log.gz",
	}
	if strings.Join(names(got), " ") != strings.Join(want, " ") {
		t.Fatalf("got %v, want %v", names(got), want)
	}
	first := got[0]
	if first.ModifiedAt != "2026-10-03T15:00:00Z" || first.Size != 4 {
		t.Fatalf("RFC 3339 in UTC and the size on disk: %+v", first)
	}
	if got[1].Size != int64(len(gz(t, "a\n"))) {
		t.Fatalf("a dated log's size is its compressed one: %+v", got[1])
	}
}

func TestListRunLogsCapsEachKindAtTheNewest(t *testing.T) {
	r := newRunLogRig(t)
	for i := range RunLogsPerKind + 5 {
		at := runLogsBase.Add(time.Duration(i) * time.Hour)
		r.write(fmt.Sprintf("logs/2026-09-%02d-%d.log.gz", i%28+1, i), gz(t, "x\n"), at)
		r.write(fmt.Sprintf("crash-reports/crash-%03d.txt", i), []byte("x"), at)
	}
	got := r.list()
	logs, crashes := 0, 0
	for _, l := range got {
		if l.Kind == models.RunLogKindLog {
			logs++
		} else {
			crashes++
		}
	}
	if logs != RunLogsPerKind || crashes != RunLogsPerKind {
		t.Fatalf("%d logs and %d crash reports, want %d of each", logs, crashes, RunLogsPerKind)
	}
	// The oldest five of each are the ones left out.
	for _, l := range got {
		if strings.HasPrefix(l.Name, "crash-00") && l.Name < "crash-005" {
			t.Fatalf("an old crash report was listed: %s", l.Name)
		}
	}
}

func TestListRunLogsOfAnInstanceWithNoGameFolderOrNoLogsIsEmpty(t *testing.T) {
	for name, instance := range map[string]string{
		"no instance path":   "",
		"no game folder":     t.TempDir(),
		"no logs or reports": func() string { d := t.TempDir(); _ = os.MkdirAll(filepath.Join(d, "minecraft"), 0o755); return d }(),
	} {
		got, err := ListRunLogs(instance)
		if err != nil || got == nil || len(got) != 0 {
			t.Errorf("%s: got %v, %v; want an empty list and no error", name, got, err)
		}
	}
}

func TestListRunLogsReadsAnOlderInstancesDotMinecraft(t *testing.T) {
	instance := t.TempDir()
	if err := os.MkdirAll(filepath.Join(instance, ".minecraft", "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instance, ".minecraft", "logs", "latest.log"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ListRunLogs(instance)
	if err != nil || len(got) != 1 || got[0].Name != "latest.log" {
		t.Fatalf("got %v, %v", got, err)
	}
}

// The crash mark is a rule over file times: a crash report written after the
// previous log was archived (when the run began) and by the end of the log.
func TestACrashReportMarksTheLogOfTheRunItWasWrittenIn(t *testing.T) {
	r := newRunLogRig(t)
	// Run 1 crashed at 13:10; its log was archived when run 2 began at 14:00.
	// Run 2 closed cleanly and was archived when run 3 began at 16:00. Run 3,
	// latest.log, crashed at 17:20 and its last write was at 17:20:30.
	r.write("logs/2026-10-03-1.log.gz", gz(t, "run 1\n"), runLogsBase.Add(2*time.Hour))
	r.write("logs/2026-10-03-2.log.gz", gz(t, "run 2\n"), runLogsBase.Add(4*time.Hour))
	r.write("logs/latest.log", []byte("run 3\n"), runLogsBase.Add(5*time.Hour+20*time.Minute+30*time.Second))
	r.write("crash-reports/crash-run1.txt", []byte("x"), runLogsBase.Add(70*time.Minute))
	r.write("crash-reports/crash-run3.txt", []byte("x"), runLogsBase.Add(5*time.Hour+20*time.Minute))

	crashed := map[string]bool{}
	for _, l := range r.list() {
		if l.Kind == models.RunLogKindLog {
			crashed[l.Name] = l.Crashed
		} else if l.Crashed {
			t.Fatalf("a crash report is never marked: %+v", l)
		}
	}
	want := map[string]bool{"2026-10-03-1.log.gz": true, "2026-10-03-2.log.gz": false, "latest.log": true}
	for name, w := range want {
		if crashed[name] != w {
			t.Errorf("%s crashed = %v, want %v (all: %v)", name, crashed[name], w, crashed)
		}
	}
}

func TestTheCrashWindowEndsTwoMinutesAfterTheLogAndOpensWhenThePreviousLogWasArchived(t *testing.T) {
	cases := map[string]struct {
		crashAt time.Duration
		want    bool
	}{
		"just after the log's last write":          {3*time.Hour + time.Minute, true},
		"past the slack":                           {3*time.Hour + 3*time.Minute, false},
		"at the moment the older log was archived": {time.Hour, false},
		"a second after it":                        {time.Hour + time.Second, true},
		"a day before the oldest log began":        {-24*time.Hour - time.Minute, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := newRunLogRig(t)
			r.write("logs/2026-10-03-1.log.gz", gz(t, "old\n"), runLogsBase.Add(time.Hour))
			r.write("logs/latest.log", []byte("x\n"), runLogsBase.Add(3*time.Hour))
			r.write("crash-reports/crash-x.txt", []byte("x"), runLogsBase.Add(c.crashAt))
			var latest, dated bool
			for _, l := range r.list() {
				switch l.Name {
				case "latest.log":
					latest = l.Crashed
				case "2026-10-03-1.log.gz":
					dated = l.Crashed
				}
			}
			if name == "a day before the oldest log began" {
				// Before the oldest log's own window: no log claims it.
				if latest || dated {
					t.Fatalf("latest %v, dated %v", latest, dated)
				}
				return
			}
			if latest != c.want {
				t.Fatalf("latest.log crashed = %v, want %v", latest, c.want)
			}
		})
	}
}

func TestReadsAPlainLogFromItsEndAndTheChunkBeforeIt(t *testing.T) {
	r := newRunLogRig(t)
	var sb strings.Builder
	line := strings.Repeat("x", 99) + "\n"
	lines := 3 * models.RunLogChunkBytes / 100
	for range lines {
		sb.WriteString(line)
	}
	r.write("logs/2026-10-03-1.log", []byte("ignored"), runLogsBase) // not a listed name
	r.write("logs/latest.log", []byte(sb.String()), runLogsBase)

	tail, err := r.read(models.RunLogKindLog, "latest.log", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !tail.Truncated || tail.Offset == 0 || tail.Size != int64(sb.Len()) {
		t.Fatalf("the end of a long file is truncated and says where it begins: %+v", tail)
	}
	if len(tail.Text) > models.RunLogChunkBytes || !strings.HasSuffix(tail.Text, "\n") || !strings.HasPrefix(tail.Text, "xxxx") {
		t.Fatalf("whole lines within the budget: %d bytes", len(tail.Text))
	}
	if tail.Offset%100 != 0 {
		t.Fatalf("a chunk starts at a line start: %d", tail.Offset)
	}
	if tail.Lines != strings.Count(tail.Text, "\n") {
		t.Fatalf("lines %d", tail.Lines)
	}
	// Walk back to the start: the chunks tile the file with no gap or overlap.
	total := len(tail.Text)
	before := tail.Offset
	for before > 0 {
		chunk, err := r.read(models.RunLogKindLog, "latest.log", before)
		if err != nil {
			t.Fatal(err)
		}
		if chunk.Offset >= before {
			t.Fatalf("no progress: %d from %d", chunk.Offset, before)
		}
		total += len(chunk.Text)
		before = chunk.Offset
		if chunk.Truncated != (chunk.Offset > 0) {
			t.Fatalf("truncated %v at offset %d", chunk.Truncated, chunk.Offset)
		}
	}
	if total != sb.Len() {
		t.Fatalf("chunks add up to %d bytes, the file has %d", total, sb.Len())
	}
}

func TestAShortLogIsReadWholeAndNotTruncated(t *testing.T) {
	r := newRunLogRig(t)
	r.write("logs/latest.log", []byte("one\ntwo\n"), runLogsBase)
	got, err := r.read(models.RunLogKindLog, "latest.log", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "one\ntwo\n" || got.Truncated || got.Offset != 0 || got.Lines != 2 || got.Size != 8 {
		t.Fatalf("got %+v", got)
	}
	// An offset past the end reads from the end, as 0 does.
	again, err := r.read(models.RunLogKindLog, "latest.log", 999)
	if err != nil || again.Text != got.Text {
		t.Fatalf("got %+v, %v", again, err)
	}
}

// latest.log is open for writing while it is read: a half-written last line is
// not shown, since half a home path would slip past the redactor.
func TestTheLiveLogsHalfWrittenLastLineIsLeftOut(t *testing.T) {
	r := newRunLogRig(t)
	r.write("logs/latest.log", []byte("one\ntwo\nC:\\Users\\san"), runLogsBase)
	got, err := r.read(models.RunLogKindLog, "latest.log", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "one\ntwo\n" {
		t.Fatalf("got %q", got.Text)
	}
	// An archived log is whole: its last line stays whether or not it ends in a newline.
	r.write("logs/2026-10-03-1.log.gz", gz(t, "one\ntwo"), runLogsBase)
	arch, err := r.read(models.RunLogKindLog, "2026-10-03-1.log.gz", 0)
	if err != nil || arch.Text != "one\ntwo" || arch.Lines != 2 {
		t.Fatalf("got %+v, %v", arch, err)
	}
}

func TestReadsAGzippedLogUnpackedWithItsEndAndTheChunkBefore(t *testing.T) {
	r := newRunLogRig(t)
	var sb strings.Builder
	for i := range 2 * models.RunLogChunkBytes / 20 {
		fmt.Fprintf(&sb, "line %013d\n", i) // 19 bytes
	}
	r.write("logs/2026-10-03-1.log.gz", gz(t, sb.String()), runLogsBase)

	tail, err := r.read(models.RunLogKindLog, "2026-10-03-1.log.gz", 0)
	if err != nil {
		t.Fatal(err)
	}
	if tail.Size != int64(sb.Len()) || !tail.Truncated || !strings.HasPrefix(tail.Text, "line ") {
		t.Fatalf("unpacked size and a tail from a whole line: %+v", tail.Size)
	}
	if !strings.HasSuffix(tail.Text, fmt.Sprintf("line %013d\n", 2*models.RunLogChunkBytes/20-1)) {
		t.Fatal("the tail ends with the last line")
	}
	earlier, err := r.read(models.RunLogKindLog, "2026-10-03-1.log.gz", tail.Offset)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(sb.String()[:tail.Offset], earlier.Text) || earlier.Offset >= tail.Offset {
		t.Fatalf("the earlier chunk ends where the tail begins: %d, %d", earlier.Offset, tail.Offset)
	}
}

func TestReadsACrashReport(t *testing.T) {
	r := newRunLogRig(t)
	r.write("crash-reports/crash-2026-10-03_12.30.00-client.txt", []byte("---- Minecraft Crash Report ----\nDescription: boom\n"), runLogsBase)
	got, err := r.read(models.RunLogKindCrash, "crash-2026-10-03_12.30.00-client.txt", 0)
	if err != nil || !strings.Contains(got.Text, "Description: boom") || got.Kind != models.RunLogKindCrash {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestAReadRefusesANameThatIsNotOneTheListingProduces(t *testing.T) {
	r := newRunLogRig(t)
	r.write("logs/latest.log", []byte("x\n"), runLogsBase)
	r.write("logs/secret.txt", []byte("not a log\n"), runLogsBase)
	r.write("logs/debug.log", []byte("not listed\n"), runLogsBase)
	if err := os.WriteFile(filepath.Join(r.instance, "instance.cfg"), []byte("[General]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ kind, name string }{
		{models.RunLogKindLog, "../instance.cfg"},
		{models.RunLogKindLog, "..\\..\\instance.cfg"},
		{models.RunLogKindLog, "../logs/latest.log"},
		{models.RunLogKindLog, "logs/latest.log"},
		{models.RunLogKindLog, "./latest.log"},
		{models.RunLogKindLog, "..log.gz"},
		{models.RunLogKindLog, "a..b.log.gz"},
		{models.RunLogKindLog, ".log.gz"},
		{models.RunLogKindLog, "latest.log:stream"},
		{models.RunLogKindLog, "C:\\Windows\\win.ini"},
		{models.RunLogKindLog, "/etc/passwd"},
		{models.RunLogKindLog, ""},
		{models.RunLogKindLog, "secret.txt"},
		{models.RunLogKindLog, "debug.log"},
		{models.RunLogKindLog, "missing.log.gz"},
		{models.RunLogKindLog, "latest.log\x00.txt"},
		{models.RunLogKindLog, strings.Repeat("a", 200) + ".log.gz"},
		{models.RunLogKindCrash, "latest.log"},
		{models.RunLogKindCrash, "../instance.cfg"},
		{models.RunLogKindCrash, "missing.txt"},
		{"config", "latest.log"},
		{"", "latest.log"},
		{"logs", "latest.log"},
	}
	for _, c := range cases {
		got, err := r.read(c.kind, c.name, 0)
		if !errors.Is(err, ErrRunLogName) || got.Text != "" {
			t.Errorf("%q %q: got %+v, %v; want ErrRunLogName", c.kind, c.name, got, err)
		}
	}
}

func TestAReadRefusesWhatIsNotAFileOrHasNoGameFolder(t *testing.T) {
	r := newRunLogRig(t)
	if err := os.Mkdir(filepath.Join(r.game, "logs", "dir.log.gz"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := r.read(models.RunLogKindLog, "dir.log.gz", 0); !errors.Is(err, ErrRunLogName) {
		t.Fatalf("a folder: %v", err)
	}
	if _, err := ReadRunLog(t.TempDir(), models.RunLogKindLog, "latest.log", 0, runLogRedactor()); !errors.Is(err, ErrRunLogName) {
		t.Fatalf("no game folder: %v", err)
	}
	if _, err := ReadRunLog(r.instance, models.RunLogKindLog, "latest.log", 0, nil); err == nil {
		t.Fatal("without a redactor nothing is read")
	}
}

// A link in the logs folder is not listed and is not read, whatever it points at.
func TestASymlinkOutOfTheFolderIsNeitherListedNorRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks needs a privilege Windows test runners lack; macOS CI runs this")
	}
	r := newRunLogRig(t)
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(r.game, "crash-reports", "crash-link.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(r.game, "logs", "latest.log")); err != nil {
		t.Fatal(err)
	}
	if got := r.list(); len(got) != 0 {
		t.Fatalf("links are not listed: %v", names(got))
	}
	for _, c := range []struct{ kind, name string }{{models.RunLogKindCrash, "crash-link.txt"}, {models.RunLogKindLog, "latest.log"}} {
		if got, err := r.read(c.kind, c.name, 0); err == nil || strings.Contains(got.Text, "secret") {
			t.Fatalf("%s: read through a link: %+v, %v", c.name, got, err)
		}
	}
	// A whole folder that is a link out of the game folder is refused by the Root.
	if err := os.RemoveAll(filepath.Join(r.game, "logs")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(outside), filepath.Join(r.game, "logs")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(outside), "latest.log"), []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := r.read(models.RunLogKindLog, "latest.log", 0); err == nil || strings.Contains(got.Text, "secret") {
		t.Fatalf("read through a linked folder: %+v, %v", got, err)
	}
	if _, err := ListRunLogs(r.instance); err == nil {
		t.Fatal("listing through a linked folder")
	}
}

// A gzip that unpacks to more than the cap is refused, never unpacked whole.
func TestAGzipBombBeyondTheCapIsRefused(t *testing.T) {
	r := newRunLogRig(t)
	bomb := gz(t, strings.Repeat("0", 64<<10))
	if len(bomb) > 1<<10 {
		t.Fatalf("the fixture should compress well: %d", len(bomb))
	}
	r.write("logs/2026-10-03-1.log.gz", bomb, runLogsBase)
	_, err := readRunLog(r.instance, models.RunLogKindLog, "2026-10-03-1.log.gz", 0, runLogRedactor(), 4<<10)
	if !errors.Is(err, ErrRunLogTooLarge) {
		t.Fatalf("got %v", err)
	}
	// Within the cap it reads.
	if got, err := readRunLog(r.instance, models.RunLogKindLog, "2026-10-03-1.log.gz", 0, runLogRedactor(), 64<<10); err != nil || got.Size != 64<<10 {
		t.Fatalf("got %+v, %v", got.Size, err)
	}
	// The real cap is far above any log a pack writes and far below memory.
	if maxUnpackedRunLog < 64<<20 || maxUnpackedRunLog > 1<<30 {
		t.Fatalf("cap %d", maxUnpackedRunLog)
	}
}

func TestACorruptGzipIsAnErrorNotAPanic(t *testing.T) {
	r := newRunLogRig(t)
	r.write("logs/2026-10-03-1.log.gz", []byte("this is not gzip"), runLogsBase)
	if _, err := r.read(models.RunLogKindLog, "2026-10-03-1.log.gz", 0); err == nil {
		t.Fatal("a corrupt file reads")
	}
	whole := gz(t, strings.Repeat("line\n", 1000))
	r.write("logs/2026-10-03-2.log.gz", whole[:len(whole)/2], runLogsBase)
	if _, err := r.read(models.RunLogKindLog, "2026-10-03-2.log.gz", 0); err == nil {
		t.Fatal("a cut file reads")
	}
}

// What leaves Go is masked: the home path, the OS user, the server, an IP and
// the in-game name the log gives, in a chunk that has the line and in one that
// has not.
func TestAReadMasksWhatIdentifiesThePlayer(t *testing.T) {
	r := newRunLogRig(t)
	head := "[12:00:00] [main/INFO]: Setting user: Alex_the_Great\n" +
		"[12:00:01] [main/INFO]: Game dir C:\\Users\\sandro\\AppData\\Roaming\\PrismLauncher\n"
	filler := strings.Repeat("[12:00:02] [main/INFO]: padding padding padding padding padding\n", models.RunLogChunkBytes/62+10)
	tailLine := "[12:30:00] [Render thread/INFO]: Alex_the_Great joined play.kapitel.example:25565 from 192.168.1.20 as sandro\n"
	r.write("logs/latest.log", []byte(head+filler+tailLine), runLogsBase)

	got, err := r.read(models.RunLogKindLog, "latest.log", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Truncated || strings.Contains(got.Text, "Setting user") {
		t.Fatal("the fixture's tail should not hold the name's line")
	}
	for _, leak := range []string{"Alex_the_Great", "play.kapitel.example", "192.168.1.20", "sandro"} {
		if strings.Contains(got.Text, leak) {
			t.Errorf("%q leaked: %s", leak, got.Text[len(got.Text)-200:])
		}
	}
	if !strings.Contains(got.Text, "[player] joined [server] from [ip] as [user]") {
		t.Fatalf("got %s", got.Text[len(got.Text)-200:])
	}
	// The whole of it, chunk by chunk, leaks nothing either.
	first, err := r.read(models.RunLogKindLog, "latest.log", got.Offset)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(first.Text, "Alex_the_Great") || strings.Contains(first.Text, `C:\Users\sandro`) {
		t.Fatalf("the earlier chunk leaked: %s", first.Text[:200])
	}
	if !strings.Contains(first.Text, "Setting user: [player]") || !strings.Contains(first.Text, "[home]") {
		t.Fatalf("got %s", first.Text[:200])
	}
}

// A crash report has no "Setting user" line: the name comes from latest.log.
func TestACrashReportsPlayerNameIsLearnedFromTheLatestLog(t *testing.T) {
	r := newRunLogRig(t)
	r.write("logs/latest.log", []byte("[12:00:00] [main/INFO]: Setting user: Alex_the_Great\n"), runLogsBase)
	r.write("crash-reports/crash-x.txt", []byte("Player Count: 1 / 8; [LocalPlayer['Alex_the_Great'/12, l='ClientLevel']]\nat C:\\Users\\sandro\\x\n"), runLogsBase)
	got, err := r.read(models.RunLogKindCrash, "crash-x.txt", 0)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.Text, "Alex_the_Great") || strings.Contains(got.Text, "sandro") {
		t.Fatalf("got %s", got.Text)
	}
}

func TestAChunkOfOneVeryLongLineIsKeptFromItsStart(t *testing.T) {
	r := newRunLogRig(t)
	r.write("logs/latest.log", []byte("start\n"+strings.Repeat("y", 3*models.RunLogChunkBytes)+"\n"), runLogsBase)
	got, err := r.read(models.RunLogKindLog, "latest.log", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Text == "" || !got.Truncated || len(got.Text) > models.RunLogChunkBytes {
		t.Fatalf("a line longer than the chunk is cut, not dropped: %d bytes, truncated %v", len(got.Text), got.Truncated)
	}
}

func TestRunLogNameValidity(t *testing.T) {
	ok := map[string][]string{
		models.RunLogKindLog:   {"latest.log", "2026-10-03-1.log.gz", "2026-10-03-12.log.gz"},
		models.RunLogKindCrash: {"crash-2026-10-03_12.30.00-client.txt"},
	}
	for kind, list := range ok {
		for _, name := range list {
			if !validRunLogName(kind, name) {
				t.Errorf("%s %q should be valid", kind, name)
			}
		}
	}
	// NeoForge's debug log of the same run, and any other archive name, is not listed.
	for _, name := range []string{"debug-1.log.gz", "debug.log", "2026-10-03.log.gz", "x-2026-10-03-1.log.gz"} {
		if validRunLogName(models.RunLogKindLog, name) {
			t.Errorf("%q is not a run's log", name)
		}
	}
	if validRunLogName(models.RunLogKindLog, "crash-x.txt") || validRunLogName(models.RunLogKindCrash, "latest.log") {
		t.Error("a name is valid for its own kind only")
	}
}
