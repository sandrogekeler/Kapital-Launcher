package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"kapital/backend/models"
)

// liveRig is a LiveLog paced by the test: each tick it sends is one look at
// the file, and the look reports when it is over.
type liveRig struct {
	*runLogRig
	live   *LiveLog
	ticks  []chan time.Time
	polled chan struct{}
	mu     sync.Mutex
	events []models.LiveLogEvent
}

func newLiveRig(t *testing.T) *liveRig {
	t.Helper()
	rig := &liveRig{runLogRig: newRunLogRig(t), polled: make(chan struct{})}
	rig.live = &LiveLog{
		newTick: func() (<-chan time.Time, func()) {
			tick := make(chan time.Time)
			rig.mu.Lock()
			rig.ticks = append(rig.ticks, tick)
			rig.mu.Unlock()
			return tick, func() {}
		},
		afterPoll: func() { rig.polled <- struct{}{} },
	}
	t.Cleanup(rig.live.Shutdown)
	return rig
}

func (r *liveRig) emit(e models.LiveLogEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *liveRig) start(chapter string) (models.RunLogText, error) {
	return r.live.Start(context.Background(), chapter, r.instance, runLogRedactor(), r.emit)
}

// look makes the follower of the n'th Start look at the file once, and returns
// when the look is over.
func (r *liveRig) look(n int) {
	r.t.Helper()
	r.mu.Lock()
	tick := r.ticks[n]
	r.mu.Unlock()
	select {
	case tick <- time.Time{}:
	case <-time.After(5 * time.Second):
		r.t.Fatal("the follower is not looking at the file")
	}
	select {
	case <-r.polled:
	case <-time.After(5 * time.Second):
		r.t.Fatal("the look did not end")
	}
}

// taken returns the events so far and forgets them.
func (r *liveRig) taken() []models.LiveLogEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.events
	r.events = nil
	return out
}

func (r *liveRig) appendLog(text string) {
	r.t.Helper()
	f, err := os.OpenFile(filepath.Join(r.game, "logs", "latest.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		r.t.Fatal(err)
	}
	if _, err := f.WriteString(text); err != nil {
		r.t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		r.t.Fatal(err)
	}
}

func (r *liveRig) replaceLog(text string) {
	r.t.Helper()
	if err := os.WriteFile(filepath.Join(r.game, "logs", "latest.log"), []byte(text), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func TestTheLiveLogStartsWithWhatAReadOfLatestLogGives(t *testing.T) {
	r := newLiveRig(t)
	r.replaceLog("[main/INFO]: Game dir C:\\Users\\sandro\\x\n[main/INFO]: Setting user: Steve\nhalf a li")
	got, err := r.start("frangfurd")
	if err != nil {
		t.Fatal(err)
	}
	want, err := r.read(models.RunLogKindLog, "latest.log", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v, want what ReadRunLog returns, %+v", got, want)
	}
	if strings.Contains(got.Text, "sandro") || strings.Contains(got.Text, "Steve") || strings.Contains(got.Text, "half") {
		t.Fatalf("masked, whole lines only: %q", got.Text)
	}
	// Nothing is repeated: the lines above were the start, not news.
	r.look(0)
	if ev := r.taken(); len(ev) != 0 {
		t.Fatalf("got %+v before anything was appended", ev)
	}
}

func TestAppendedLinesArriveMaskedAndAPartialLineWaitsForItsNewline(t *testing.T) {
	r := newLiveRig(t)
	r.replaceLog("[main/INFO]: Setting user: Steve\n")
	if _, err := r.start("frangfurd"); err != nil {
		t.Fatal(err)
	}
	r.appendLog("[main/INFO]: Steve joined from C:\\Users\\sandro\\w\nand a half li")
	r.look(0)
	ev := r.taken()
	if len(ev) != 1 || ev[0].ChapterID != "frangfurd" || ev[0].Reset {
		t.Fatalf("got %+v", ev)
	}
	// The in-game name is learned from the log head, as a read does.
	if ev[0].Lines != "[main/INFO]: [player] joined from [home]\\w\n" {
		t.Fatalf("got %q", ev[0].Lines)
	}

	// The half line is kept until its newline arrives, then goes whole.
	r.look(0)
	if ev := r.taken(); len(ev) != 0 {
		t.Fatalf("a partial line was sent: %+v", ev)
	}
	r.appendLog("ne, play.kapitel.example:25565\n")
	r.look(0)
	ev = r.taken()
	if len(ev) != 1 || ev[0].Lines != "and a half line, [server]\n" {
		t.Fatalf("got %+v", ev)
	}
}

func TestTheLiveLogLearnsThePlayerFromTheLinesThatArrive(t *testing.T) {
	r := newLiveRig(t)
	r.replaceLog("")
	if _, err := r.start("frangfurd"); err != nil {
		t.Fatal(err)
	}
	r.appendLog("[main/INFO]: Setting user: Alex\n[main/INFO]: <Alex> hi\n")
	r.look(0)
	r.appendLog("[main/INFO]: Alex left\n")
	r.look(0)
	var all string
	for _, e := range r.taken() {
		all += e.Lines
	}
	if all != "[main/INFO]: Setting user: [player]\n[main/INFO]: <[player]> hi\n[main/INFO]: [player] left\n" {
		t.Fatalf("got %q", all)
	}
}

func TestAFileThatShrankIsAResetWithTheNewFilesEnd(t *testing.T) {
	r := newLiveRig(t)
	r.replaceLog("[05Oct2026 10:00:00.000] [main/INFO]: first run, a long enough line one\n[05Oct2026 10:00:01.000] [main/INFO]: first run, line two\n")
	if _, err := r.start("frangfurd"); err != nil {
		t.Fatal(err)
	}
	r.replaceLog("[05Oct2026 11:00:00.000] [main/INFO]: second\n")
	r.look(0)
	ev := r.taken()
	if len(ev) != 1 || !ev[0].Reset || ev[0].Lines != "[05Oct2026 11:00:00.000] [main/INFO]: second\n" {
		t.Fatalf("got %+v", ev)
	}
	// And the follower goes on from the end of the new file.
	r.appendLog("[05Oct2026 11:00:01.000] [main/INFO]: third\n")
	r.look(0)
	ev = r.taken()
	if len(ev) != 1 || ev[0].Reset || ev[0].Lines != "[05Oct2026 11:00:01.000] [main/INFO]: third\n" {
		t.Fatalf("got %+v", ev)
	}
}

// A new run whose file is already longer than the old one at the next look is
// still a new file: it is told by its first bytes.
func TestAReplacedFileThatIsLongerIsStillAReset(t *testing.T) {
	r := newLiveRig(t)
	r.replaceLog("[05Oct2026 10:00:00.000] [main/INFO]: old\n")
	if _, err := r.start("frangfurd"); err != nil {
		t.Fatal(err)
	}
	next := "[05Oct2026 12:34:56.789] [main/INFO]: new run\n[05Oct2026 12:34:57.000] [main/INFO]: and more of it\n"
	r.replaceLog(next)
	r.look(0)
	ev := r.taken()
	if len(ev) != 1 || !ev[0].Reset || ev[0].Lines != next {
		t.Fatalf("got %+v", ev)
	}
}

func TestAnEmptyNewFileIsAResetThatClearsTheView(t *testing.T) {
	r := newLiveRig(t)
	r.replaceLog("[05Oct2026 10:00:00.000] [main/INFO]: old\n")
	if _, err := r.start("frangfurd"); err != nil {
		t.Fatal(err)
	}
	r.replaceLog("")
	r.look(0)
	ev := r.taken()
	if len(ev) != 1 || !ev[0].Reset || ev[0].Lines != "" {
		t.Fatalf("got %+v", ev)
	}
	r.appendLog("[05Oct2026 12:00:00.000] [main/INFO]: first line\n")
	r.look(0)
	ev = r.taken()
	if len(ev) != 1 || ev[0].Reset || ev[0].Lines != "[05Oct2026 12:00:00.000] [main/INFO]: first line\n" {
		t.Fatalf("got %+v", ev)
	}
}

func TestALatestLogThatIsNotThereYetIsWaitedFor(t *testing.T) {
	r := newLiveRig(t)
	got, err := r.start("frangfurd")
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "" || got.Name != "latest.log" || got.Kind != models.RunLogKindLog {
		t.Fatalf("got %+v", got)
	}
	r.look(0)
	if ev := r.taken(); len(ev) != 0 {
		t.Fatalf("got %+v", ev)
	}
	r.appendLog("[main/INFO]: it begins\n")
	r.look(0)
	ev := r.taken()
	if len(ev) != 1 || ev[0].Reset || ev[0].Lines != "[main/INFO]: it begins\n" {
		t.Fatalf("got %+v", ev)
	}
}

func TestAnInstanceWithNoGameFolderIsWaitedForToo(t *testing.T) {
	r := newLiveRig(t)
	got, err := r.live.Start(context.Background(), "frangfurd", t.TempDir(), runLogRedactor(), r.emit)
	if err != nil || got.Text != "" {
		t.Fatalf("got %+v, %v", got, err)
	}
	r.look(0)
}

// One look carries at most an event's worth; the rest follows, nothing is lost.
func TestWhatOneLookReadsIsSentInCappedEventsWithoutLoss(t *testing.T) {
	r := newLiveRig(t)
	r.replaceLog("")
	if _, err := r.start("frangfurd"); err != nil {
		t.Fatal(err)
	}
	line := strings.Repeat("x", 99) + "\n"
	count := 3 * models.LiveLogEventBytes / len(line)
	r.appendLog(strings.Repeat(line, count))
	r.look(0)
	var got strings.Builder
	events := r.taken()
	for _, e := range events {
		if len(e.Lines) > models.LiveLogEventBytes {
			t.Fatalf("an event of %d bytes, the cap is %d", len(e.Lines), models.LiveLogEventBytes)
		}
		if !strings.HasSuffix(e.Lines, "\n") {
			t.Fatalf("an event ends inside a line")
		}
		got.WriteString(e.Lines)
	}
	if len(events) < 3 || got.String() != strings.Repeat(line, count) {
		t.Fatalf("%d events, %d bytes of %d", len(events), got.Len(), count*len(line))
	}
}

func TestALineLongerThanAnEventGoesAsItStandsAndTheRestFollows(t *testing.T) {
	r := newLiveRig(t)
	r.replaceLog("")
	if _, err := r.start("frangfurd"); err != nil {
		t.Fatal(err)
	}
	r.appendLog(strings.Repeat("y", models.LiveLogEventBytes+10))
	r.look(0)
	ev := r.taken()
	if len(ev) != 1 || len(ev[0].Lines) != models.LiveLogEventBytes {
		t.Fatalf("got %d events", len(ev))
	}
	r.appendLog("\n")
	r.look(0)
	ev = r.taken()
	if len(ev) != 1 || ev[0].Lines != strings.Repeat("y", 10)+"\n" {
		t.Fatalf("got %+v", ev)
	}
}

func TestAResetOfAFileLargerThanAnEventCarriesItsEndFromAWholeLine(t *testing.T) {
	r := newLiveRig(t)
	r.replaceLog("[05Oct2026 10:00:00.000] [main/INFO]: old\n")
	if _, err := r.start("frangfurd"); err != nil {
		t.Fatal(err)
	}
	line := "[05Oct2026 12:00:00.000] [main/INFO]: " + strings.Repeat("z", 60) + "\n"
	r.replaceLog(strings.Repeat(line, 2*models.LiveLogEventBytes/len(line)))
	r.look(0)
	ev := r.taken()
	if len(ev) != 1 || !ev[0].Reset || len(ev[0].Lines) > models.LiveLogEventBytes ||
		len(ev[0].Lines) < models.LiveLogEventBytes-len(line) || !strings.HasPrefix(ev[0].Lines, "[05Oct2026") {
		t.Fatalf("got %d events, %d bytes", len(ev), len(ev[0].Lines))
	}
}

func TestStopStopsTheFollowerAndWaitsForIt(t *testing.T) {
	r := newLiveRig(t)
	r.replaceLog("a\n")
	if _, err := r.start("frangfurd"); err != nil {
		t.Fatal(err)
	}
	r.live.Stop("frangfurd")
	r.appendLog("b\n")
	select {
	case r.ticks[0] <- time.Time{}:
		t.Fatal("the follower is still looking after Stop")
	case <-time.After(50 * time.Millisecond):
	}
	if ev := r.taken(); len(ev) != 0 {
		t.Fatalf("got %+v after Stop", ev)
	}
	// Stopping what is stopped is no harm.
	r.live.Stop("frangfurd")
}

func TestStopOfAnotherChapterLeavesTheFollowerAlone(t *testing.T) {
	r := newLiveRig(t)
	r.replaceLog("a\n")
	if _, err := r.start("frangfurd"); err != nil {
		t.Fatal(err)
	}
	r.live.Stop("luxemburg")
	r.appendLog("b\n")
	r.look(0)
	if ev := r.taken(); len(ev) != 1 || ev[0].Lines != "b\n" {
		t.Fatalf("got %+v", ev)
	}
}

func TestOnlyOneLogIsFollowedAtATime(t *testing.T) {
	r := newLiveRig(t)
	r.replaceLog("a\n")
	if _, err := r.start("frangfurd"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.start("luxemburg"); err != nil {
		t.Fatal(err)
	}
	r.appendLog("b\n")
	// The first follower is gone: nothing receives its ticks.
	select {
	case r.ticks[0] <- time.Time{}:
		t.Fatal("the first follower outlived the second")
	case <-time.After(50 * time.Millisecond):
	}
	r.look(1)
	ev := r.taken()
	if len(ev) != 1 || ev[0].ChapterID != "luxemburg" || ev[0].Lines != "b\n" {
		t.Fatalf("got %+v", ev)
	}
	// A Stop of the chapter before cannot end the one that took its place.
	r.live.Stop("frangfurd")
	r.appendLog("c\n")
	r.look(1)
	if ev := r.taken(); len(ev) != 1 || ev[0].Lines != "c\n" {
		t.Fatalf("got %+v", ev)
	}
}

func TestACancelledContextStopsTheFollower(t *testing.T) {
	r := newLiveRig(t)
	r.replaceLog("a\n")
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := r.live.Start(ctx, "frangfurd", r.instance, runLogRedactor(), r.emit); err != nil {
		t.Fatal(err)
	}
	cancel()
	r.live.mu.Lock()
	done := r.live.done
	r.live.mu.Unlock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the follower outlived its context")
	}
}

func TestAnythingButLatestLogIsNeverFollowed(t *testing.T) {
	r := newLiveRig(t)
	r.replaceLog("a\n")
	r.write("logs/2026-10-03-1.log.gz", gz(t, "old\n"), runLogsBase)
	r.write("crash-reports/crash-a.txt", []byte("boom\n"), runLogsBase)
	for _, c := range []struct{ kind, name string }{
		{models.RunLogKindLog, "2026-10-03-1.log.gz"},
		{models.RunLogKindCrash, "crash-a.txt"},
		{models.RunLogKindCrash, "latest.log"},
		{models.RunLogKindLog, "../instance.cfg"},
		{models.RunLogKindLog, "../../x.log"},
		{models.RunLogKindLog, `..\latest.log`},
		{models.RunLogKindLog, "/etc/passwd"},
		{models.RunLogKindLog, "debug.log"},
		{models.RunLogKindLog, ""},
		{"config", "latest.log"},
	} {
		_, err := r.live.start(context.Background(), "frangfurd", r.instance, liveTarget{c.kind, c.name}, runLogRedactor(), r.emit)
		if !errors.Is(err, ErrRunLogName) {
			t.Errorf("%q %q: got %v", c.kind, c.name, err)
		}
	}
	r.live.mu.Lock()
	running := r.live.cancel != nil
	r.live.mu.Unlock()
	if running {
		t.Fatal("a refused name started a follower")
	}
	if _, err := r.live.Start(context.Background(), "frangfurd", r.instance, nil, r.emit); err == nil {
		t.Fatal("without a redactor nothing is followed")
	}
}

// A latest.log that is a folder (or, off Windows, a link) is not followed.
func TestALatestLogThatIsNotARegularFileIsNotRead(t *testing.T) {
	r := newLiveRig(t)
	if err := os.Mkdir(filepath.Join(r.game, "logs", "latest.log"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := r.start("frangfurd"); !errors.Is(err, ErrRunLogName) {
		t.Fatalf("got %v", err)
	}
}

func TestAFollowerWaitsOutAFileThatDisappearsDuringARun(t *testing.T) {
	r := newLiveRig(t)
	r.replaceLog("[05Oct2026 10:00:00.000] [main/INFO]: a\n")
	if _, err := r.start("frangfurd"); err != nil {
		t.Fatal(err)
	}
	// The game archives latest.log before it creates the next.
	if err := os.Remove(filepath.Join(r.game, "logs", "latest.log")); err != nil {
		t.Fatal(err)
	}
	r.look(0)
	if ev := r.taken(); len(ev) != 0 {
		t.Fatalf("got %+v", ev)
	}
	r.replaceLog("[05Oct2026 11:00:00.000] [main/INFO]: b\n")
	r.look(0)
	ev := r.taken()
	if len(ev) != 1 || !ev[0].Reset || ev[0].Lines != "[05Oct2026 11:00:00.000] [main/INFO]: b\n" {
		t.Fatalf("got %+v", ev)
	}
}
