package services

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"kapital/backend/models"
)

func TestLaunchEstimateIsTheMeanOfEachPhaseOverTheStartsThatHaveIt(t *testing.T) {
	cases := []struct {
		name  string
		times []models.LaunchTiming
		want  map[string]int64
	}{
		{"no history, no estimate", nil, nil},
		{"a start with no phases, no estimate", []models.LaunchTiming{{PhaseMs: map[string]int64{}}}, nil},
		{"one start is its own mean", []models.LaunchTiming{
			{PhaseMs: map[string]int64{"mods": 1000, "window": 20000, "resources": 40000, "running": 50000}},
		}, map[string]int64{"mods": 1000, "window": 20000, "resources": 40000, "running": 50000}},
		{"the mean over the starts", []models.LaunchTiming{
			{PhaseMs: map[string]int64{"mods": 1000, "window": 20000, "resources": 40000, "running": 50000}},
			{PhaseMs: map[string]int64{"mods": 3000, "window": 24000, "resources": 44000, "running": 60000}},
		}, map[string]int64{"mods": 2000, "window": 22000, "resources": 42000, "running": 55000}},
		{"a key is averaged over the starts that have it, and one nobody has is left out", []models.LaunchTiming{
			{PhaseMs: map[string]int64{"mods": 1000, "resources": 40000, "running": 50000}},
			{PhaseMs: map[string]int64{"mods": 3000, "window": 24000, "running": 60000}},
			{PhaseMs: map[string]int64{"mods": 5000, "running": 70000}},
		}, map[string]int64{"mods": 3000, "window": 24000, "resources": 40000, "running": 60000}},
		{"rounded to the nearest millisecond", []models.LaunchTiming{
			{PhaseMs: map[string]int64{"mods": 1}}, {PhaseMs: map[string]int64{"mods": 2}},
		}, map[string]int64{"mods": 2}},
		{"a phase the tracker does not time is ignored", []models.LaunchTiming{
			{PhaseMs: map[string]int64{"stopping": 9}},
		}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := LaunchEstimate(c.times); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

// Every event of a run carries the estimate taken when Play was pressed, and
// the run's own timings do not move it.
func TestTrackerCarriesTheEstimateOnEveryEventOfARun(t *testing.T) {
	r := newGameRig(t)
	r.tracker.recordTiming("frangfurd", models.LaunchTiming{PhaseMs: map[string]int64{"mods": 2000, "running": 40000}})
	want := map[string]int64{"mods": 2000, "running": 40000}
	r.procs.add(200, 100, "javaw.exe", r.play.Add(7*time.Second))
	r.start()
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n")
	r.untilPhase("mods")
	r.log.append(render + "Sound engine started\n")
	r.untilPhase("running")
	r.log.append(render + "Stopping!\n")
	r.untilPhase("stopping")
	r.procs.end(200, 0)
	r.untilPhase("closed")

	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.events) != 5 {
		t.Fatalf("got %d events", len(r.events))
	}
	for _, e := range r.events {
		if !reflect.DeepEqual(e.Estimate, want) {
			t.Fatalf("%s carries %v, want %v", e.Phase, e.Estimate, want)
		}
	}
	if got := r.tracker.Latest("frangfurd").Estimate; !reflect.DeepEqual(got, want) {
		t.Fatalf("Latest carries %v", got)
	}
}

func TestTrackerHasNoEstimateForAChapterWithNoHistory(t *testing.T) {
	r := newGameRig(t)
	r.start()
	if got := r.tracker.Latest("frangfurd"); got.Estimate != nil {
		t.Fatalf("%+v", got)
	}
}

// handoverCalls counts OnHandover calls and, for a test, what had happened to
// the held window by the time each one came.
type handoverCalls struct {
	mu      sync.Mutex
	n       int
	seen    []int
	release func() int
}

func (h *handoverCalls) call() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.n++
	if h.release != nil {
		h.seen = append(h.seen, h.release())
	}
}

func (h *handoverCalls) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.n
}

func TestTrackerCallsOnHandoverOnceAndOnlyAfterTheForegroundRelease(t *testing.T) {
	r, fh := holdRig(t, true)
	calls := &handoverCalls{release: func() int { return len(fh.holder(0).releases()) }}
	r.onHandover = calls.call
	fh.block = make(chan struct{})
	r.procs.add(200, 100, "javaw.exe", r.play.Add(7*time.Second))
	r.start()
	until(t, "the hold", func() bool { return fh.count() == 1 })
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n" + render + "Reloading ResourceManager\n")
	r.untilPhase("resources")
	// The release is still under way, so the launcher must not minimise yet.
	time.Sleep(20 * time.Millisecond)
	if calls.count() != 0 {
		t.Fatal("OnHandover came before the release finished")
	}
	close(fh.block)
	until(t, "OnHandover", func() bool { return calls.count() == 1 })

	// The log then reaches running: still one handover.
	r.log.append(render + "Sound engine started\n")
	r.untilPhase("running")
	time.Sleep(20 * time.Millisecond)
	calls.mu.Lock()
	defer calls.mu.Unlock()
	if calls.n != 1 || !reflect.DeepEqual(calls.seen, []int{1}) {
		t.Fatalf("one call, after the one release: n=%d seen=%v", calls.n, calls.seen)
	}
}

func TestTrackerCallsOnHandoverAtOnceWhenNoWindowIsHeld(t *testing.T) {
	// The setting off, or no game process to hold: the phase itself is the
	// handover.
	r, _ := holdRig(t, false)
	calls := &handoverCalls{}
	r.onHandover = calls.call
	r.start()
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n" + render + "Reloading ResourceManager\n")
	r.untilPhase("resources")
	until(t, "OnHandover", func() bool { return calls.count() == 1 })
	r.log.append(render + "Sound engine started\n")
	r.untilPhase("running")
	time.Sleep(20 * time.Millisecond)
	if calls.count() != 1 {
		t.Fatalf("once per run, got %d", calls.count())
	}
}

func TestTrackerDoesNotCallOnHandoverForARunThatEndsFirst(t *testing.T) {
	r, fh := holdRig(t, true)
	calls := &handoverCalls{}
	r.onHandover = calls.call
	r.procs.add(200, 100, "javaw.exe", r.play.Add(7*time.Second))
	r.start()
	until(t, "the hold", func() bool { return fh.count() == 1 })
	r.log.write("[00:30:39] [main/INFO]: ModLauncher running\n" + render + "Backend library: LWJGL\n")
	r.untilPhase("window")
	r.procs.end(200, 1)
	r.untilPhase("crashed")
	time.Sleep(20 * time.Millisecond)
	if calls.count() != 0 {
		t.Fatalf("a crash before the reload is not a handover: %d", calls.count())
	}
}
