package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	// The lines Prism logs when the pre-launch command fails, in order. The
	// quoting is QDebug's and unconfirmed on a real file [verify]; see
	// testdata/prism/prelaunch-failed.log.
	prismPreLaunchLine  = `12.345 Critical: [launcher.task] Task "PreLaunchCommand(0x55d2 ID: {u1})" failed: "Pre-Launch command failed with code 1.\n\n" (Task::emitFailed:128)` + "\n"
	prismLaunchTaskLine = `12.346 Critical: [launcher.task] Task "LaunchTask(0x55d3 ID: {u2})" failed: "Pre-Launch command failed with code 1.\n\n" (Task::emitFailed:128)` + "\n"
	prismControllerLine = `12.346 Critical: [launcher.task] Task "LaunchController(0x55d4 ID: {u3})" failed: "Pre-Launch command failed with code 1.\n\n" (Task::emitFailed:128)` + "\n"
	prismJavaFailure    = `9.100 Critical: [launcher.task] Task "LaunchTask(0x55d3 ID: {u2})" failed: "Could not download Java 21" (Task::emitFailed:128)` + "\n"
	prismNetJobFailure  = `3.200 Critical: [launcher.task] Task "NetJob(0x55d9 ID: {u4})" failed: "Network error: timed out" (Task::emitFailed:128)` + "\n"
	prismFirstLine      = "0.011 Debug: [launcher.instance] Launching an instance (Application::launch:1042)\n"
)

// prismSyncFailure is the three lines a failed pack sync leaves, as a whole.
const prismSyncFailure = prismFirstLine + prismPreLaunchLine + prismLaunchTaskLine + prismControllerLine

// prismLog is Prism's launcher log under a data root of the test's own.
type prismLog struct {
	t    *testing.T
	path string
}

func newPrismLog(t *testing.T, r *gameRig) *prismLog {
	t.Helper()
	r.prismRoot = t.TempDir()
	path := prismLogPath(r.prismRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	return &prismLog{t: t, path: path}
}

// write replaces the log's contents, truncating it as Prism does on start.
func (p *prismLog) write(s string) {
	p.t.Helper()
	if err := os.WriteFile(p.path, []byte(s), 0o600); err != nil {
		p.t.Fatal(err)
	}
}

func (p *prismLog) append(s string) {
	p.t.Helper()
	f, err := os.OpenFile(p.path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		p.t.Fatal(err)
	}
	if _, err := f.WriteString(s); err != nil {
		p.t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		p.t.Fatal(err)
	}
}

// stepAfter moves the clock a little and steps the run once.
func (r *gameRig) stepAfter(run *gameRun, d time.Duration) bool {
	r.clock.Advance(d)
	return run.step(r.clock.Now())
}

func TestTrackerEndsAStartWhenThePackSyncFails(t *testing.T) {
	r := newGameRig(t)
	plog := newPrismLog(t, r)
	run := r.begin()
	if r.stepAfter(run, time.Second) || run.state.Phase != "starting" {
		t.Fatalf("nothing logged yet: %s", run.state.Phase)
	}
	plog.write(prismSyncFailure)
	if !r.stepAfter(run, time.Second) {
		t.Fatalf("Prism's failed launch ends the run: %s", run.state.Phase)
	}
	if run.state.Phase != "failed" || run.state.Reason != "packsync" {
		t.Fatalf("got %+v", run.state)
	}
	if r.clock.Now().Sub(r.play) >= r.tracker.startTimeout {
		t.Fatal("it ended well before the start timeout")
	}
	if r.tracker.Active("frangfurd") {
		t.Fatal("failed is not active")
	}
	got := r.tracker.Latest("frangfurd")
	if got.Phase != "failed" || got.Reason != "packsync" {
		t.Fatalf("the state held carries the reason: %+v", got)
	}
	r.mu.Lock()
	last := r.events[len(r.events)-1]
	r.mu.Unlock()
	if last.Phase != "failed" || last.Reason != "packsync" {
		t.Fatalf("the event carries the reason: %+v", last)
	}
}

func TestTrackerClassesAStepThatIsNotThePackSyncAsALaunchFailure(t *testing.T) {
	r := newGameRig(t)
	plog := newPrismLog(t, r)
	run := r.begin()
	plog.write(prismFirstLine + prismJavaFailure)
	if !r.stepAfter(run, time.Second) || run.state.Phase != "failed" || run.state.Reason != "launch" {
		t.Fatalf("got %+v", run.state)
	}
}

func TestTrackerIgnoresTasksThatDoNotStopALaunch(t *testing.T) {
	r := newGameRig(t)
	plog := newPrismLog(t, r)
	run := r.begin()
	// A background job failing, and the pre-launch task on its own: only the
	// LaunchTask's line is acted on.
	plog.write(prismFirstLine + prismNetJobFailure + prismPreLaunchLine + prismControllerLine)
	if r.stepAfter(run, time.Second) || run.state.Phase != "starting" || run.state.Reason != "" {
		t.Fatalf("got %+v", run.state)
	}
	// The line arriving in two writes, as a log being written can.
	half := len(prismLaunchTaskLine) / 2
	plog.append(prismLaunchTaskLine[:half])
	if r.stepAfter(run, time.Second) || run.state.Phase != "starting" {
		t.Fatalf("an unfinished line is not a failure: %s", run.state.Phase)
	}
	plog.append(prismLaunchTaskLine[half:])
	if !r.stepAfter(run, time.Second) || run.state.Reason != "packsync" {
		t.Fatalf("the finished line is: %+v", run.state)
	}
}

func TestTrackerDoesNotTakeAnEarlierStartsFailureForThisOne(t *testing.T) {
	t.Run("left over from before Play", func(t *testing.T) {
		r := newGameRig(t)
		plog := newPrismLog(t, r)
		plog.write(prismSyncFailure)
		run := r.begin()
		if r.stepAfter(run, time.Second) || run.state.Phase != "starting" {
			t.Fatalf("the snapshot holds it back: %s", run.state.Phase)
		}
	})
	t.Run("truncated and rewritten after Play", func(t *testing.T) {
		r := newGameRig(t)
		plog := newPrismLog(t, r)
		plog.write(prismSyncFailure + prismSyncFailure)
		run := r.begin()
		if r.stepAfter(run, time.Second) {
			t.Fatal("the old failure is not this start's")
		}
		// Prism starts: the file is cut and the new run's lines are shorter
		// than the old ones were.
		plog.write(prismFirstLine + prismJavaFailure)
		if !r.stepAfter(run, time.Second) || run.state.Reason != "launch" {
			t.Fatalf("the new failure is: %+v", run.state)
		}
	})
	t.Run("replaced by a longer log", func(t *testing.T) {
		r := newGameRig(t)
		plog := newPrismLog(t, r)
		plog.write(prismSyncFailure)
		run := r.begin()
		// Rotated on start: another first line, and more of it than before.
		plog.write("1.000 Debug: [launcher.app] Another start (Application::Application:200)\n" +
			strings.Repeat("2.000 Debug: [launcher.app] padding (Application::x:1)\n", 20) + prismJavaFailure)
		if !r.stepAfter(run, time.Second) || run.state.Reason != "launch" {
			t.Fatalf("read from the new file's start: %+v", run.state)
		}
	})
	t.Run("appended by a Prism that was already open", func(t *testing.T) {
		r := newGameRig(t)
		plog := newPrismLog(t, r)
		plog.write(prismSyncFailure)
		run := r.begin()
		if r.stepAfter(run, time.Second) {
			t.Fatal("the old failure is not this start's")
		}
		plog.append(prismJavaFailure)
		if !r.stepAfter(run, time.Second) || run.state.Reason != "launch" {
			t.Fatalf("what the running Prism logged after Play is: %+v", run.state)
		}
	})
}

func TestTrackerToleratesAPrismLogThatIsMissingOrUnreadable(t *testing.T) {
	t.Run("not written yet", func(t *testing.T) {
		r := newGameRig(t)
		newPrismLog(t, r)
		run := r.begin()
		for range 3 {
			if r.stepAfter(run, time.Second) || run.state.Phase != "starting" {
				t.Fatalf("a missing log changes nothing: %s", run.state.Phase)
			}
		}
		// Written from nothing once Prism starts.
		plog := &prismLog{t: t, path: prismLogPath(r.prismRoot)}
		plog.write(prismSyncFailure)
		if !r.stepAfter(run, time.Second) || run.state.Reason != "packsync" {
			t.Fatalf("got %+v", run.state)
		}
	})
	t.Run("gone and back between reads", func(t *testing.T) {
		r := newGameRig(t)
		plog := newPrismLog(t, r)
		plog.write(prismFirstLine)
		run := r.begin()
		r.stepAfter(run, time.Second)
		if err := os.Remove(plog.path); err != nil {
			t.Fatal(err)
		}
		if r.stepAfter(run, time.Second) {
			t.Fatal("a log that is gone changes nothing")
		}
		plog.write(prismFirstLine + prismJavaFailure)
		if !r.stepAfter(run, time.Second) || run.state.Reason != "launch" {
			t.Fatalf("got %+v", run.state)
		}
	})
	t.Run("a folder where the log should be", func(t *testing.T) {
		r := newGameRig(t)
		newPrismLog(t, r)
		if err := os.Mkdir(prismLogPath(r.prismRoot), 0o755); err != nil {
			t.Fatal(err)
		}
		run := r.begin()
		for range 3 {
			if r.stepAfter(run, time.Second) || run.state.Phase != "starting" {
				t.Fatalf("an unreadable log changes nothing: %s", run.state.Phase)
			}
		}
	})
	t.Run("no data root", func(t *testing.T) {
		r := newGameRig(t)
		run := r.begin()
		if run.prism != nil {
			t.Fatal("an empty root follows nothing")
		}
		if r.stepAfter(run, time.Second) || run.state.Phase != "starting" {
			t.Fatalf("got %s", run.state.Phase)
		}
		if SnapshotPrismLog("") != (PrismLogSnapshot{}) {
			t.Fatal("an empty root has nothing to record")
		}
	})
}

func TestTrackerStopsReadingPrismsLogOnceTheGameLogBegan(t *testing.T) {
	r := newGameRig(t)
	plog := newPrismLog(t, r)
	run := r.begin()
	r.log.write("first\n")
	if r.stepAfter(run, time.Second) || run.state.Phase != "mods" {
		t.Fatalf("a fresh game log is a game: %s", run.state.Phase)
	}
	plog.write(prismSyncFailure)
	if r.stepAfter(run, time.Second) || run.state.Phase != "mods" || run.state.Reason != "" {
		t.Fatalf("a failure after the JVM is up is the game's: %+v", run.state)
	}
}

func TestTrackerStillTimesOutWithAPrismLogThatSaysNothing(t *testing.T) {
	r := newGameRig(t)
	plog := newPrismLog(t, r)
	plog.write(prismFirstLine)
	run := r.begin()
	if r.stepAfter(run, time.Minute) {
		t.Fatal("within the time")
	}
	if !r.stepAfter(run, 11*time.Minute) || run.state.Phase != "failed" || run.state.Reason != "" {
		t.Fatalf("the timeout keeps an empty reason: %+v", run.state)
	}
}

func TestPrismTaskFailure(t *testing.T) {
	for _, c := range []struct {
		name string
		line string
		want string
		ok   bool
	}{
		{"pack sync", prismLaunchTaskLine, "packsync", true},
		{"other step", prismJavaFailure, "launch", true},
		{"quoting is not relied on", `[launcher.task] LaunchTask(0x1 ID: {x}) failed: Pre-Launch command failed with code 2`, "packsync", true},
		{"pre-launch task alone", prismPreLaunchLine, "", false},
		{"controller", prismControllerLine, "", false},
		{"background job", prismNetJobFailure, "", false},
		{"another category", `1.0 Critical: [launcher.net] Task "LaunchTask(0x1)" failed: "x"` + "\n", "", false},
		{"not a failure", `1.0 Debug: [launcher.task] Task "LaunchTask(0x1 ID: {x})" succeeded` + "\n", "", false},
		{"empty", "", "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, ok := prismTaskFailure([]byte(c.line))
			if got != c.want || ok != c.ok {
				t.Fatalf("got %q %v, want %q %v", got, ok, c.want, c.ok)
			}
		})
	}
}

// The fixture is synthetic: Prism 11.1.1's lines with the pointers, ids, names
// and paths replaced [verify against a real file].
func TestPrismLogFixtureEndsAStartAsAPackSyncFailure(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "prism", "prelaunch-failed.log"))
	if err != nil {
		t.Fatal(err)
	}
	r := newGameRig(t)
	plog := newPrismLog(t, r)
	run := r.begin()
	plog.write(string(data))
	if !r.stepAfter(run, time.Second) || run.state.Phase != "failed" || run.state.Reason != "packsync" {
		t.Fatalf("got %+v", run.state)
	}
}

func TestSnapshotPrismLog(t *testing.T) {
	root := t.TempDir()
	if snap := SnapshotPrismLog(root); snap.exists {
		t.Fatal("no log yet")
	}
	path := prismLogPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(prismSyncFailure), 0o600); err != nil {
		t.Fatal(err)
	}
	snap := SnapshotPrismLog(root)
	if !snap.exists || snap.size != int64(len(prismSyncFailure)) || !snap.head.complete {
		t.Fatalf("got %+v", snap)
	}
	// A folder where the log should be is unreadable, and not a snapshot.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if snap := SnapshotPrismLog(root); snap.exists {
		t.Fatalf("got %+v", snap)
	}
}
