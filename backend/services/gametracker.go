package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"kapital/backend/models"
)

// EventGameState is the Wails event carrying a models.GameState. Every
// payload names its chapter, and a listener filters on it.
const EventGameState = "game:state"

const (
	// gameProcInterval is how often Prism's children are looked at while the
	// game has not been found.
	gameProcInterval = time.Second
	// gameStartTimeout is how long a start may go without a game log. A first
	// start downloads Java, the libraries and the assets, and Prism's sign-in
	// can wait on the player, so it is generous.
	gameStartTimeout = 10 * time.Minute
	// gamePrismGrace is how long after the launcher's Prism has exited, with
	// no game found and no fresh log, before the start counts as failed.
	gamePrismGrace = 30 * time.Second
	// gameLogQuiet is how long the log must be still after "Stopping!" for a
	// game that cannot be watched as a process to count as closed.
	gameLogQuiet = 10 * time.Second
	// gameProcSlowInterval is how often Prism's children are looked at once the
	// loader is past reading mods and the game's Java is still not found.
	gameProcSlowInterval = 5 * time.Second
	// gamePrismGone is how long after the launcher's Prism exits, with the game
	// log already begun and no game process to wait on, the game counts as
	// gone. The managed root quits Prism when the game stops (QuitAfterGameStop),
	// so its exit is the game's; the pause lets the lookup and the log have a
	// last say.
	gamePrismGone = 2 * time.Second

	launchTimesFile = "launchtimes.json"
	launchTimesKept = 5
)

var errGameProcUnsupported = errors.New("process lookup is not supported on this OS")

// procInfo is one row of the OS's process table.
type procInfo struct {
	PID, PPID int
	Name      string
}

// gameOS is what the tracker asks of the OS, injected so a fake answers in
// tests. The real one is systemGameOS, per platform (gameproc_*.go).
type gameOS struct {
	// list returns every process.
	list func() ([]procInfo, error)
	// started is when a process was created, false when unknown.
	started func(pid int) (time.Time, bool)
	// wait blocks until the process exits or ctx is done. The exit code is
	// known only where the platform reports it.
	wait func(ctx context.Context, pid int) (code int, known bool, err error)
}

// PrismProcess is the Prism the launcher started: its pid, and a channel that
// closes when it exits.
type PrismProcess struct {
	PID    int
	Exited <-chan struct{}
}

// WatchPrism waits on the process Launch returned, so that it is reaped and
// the tracker can tell when it is gone.
func WatchPrism(p *os.Process) PrismProcess {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := p.Wait(); err != nil {
			slog.Warn("wait for prism", "error", err)
		}
	}()
	return PrismProcess{PID: p.Pid, Exited: done}
}

// TrackRequest is one launch to follow.
type TrackRequest struct {
	ChapterID string
	// InstanceDir is the chapter's Prism instance folder, where the game log
	// is.
	InstanceDir string
	Prism       PrismProcess
	// PrismExe is the base name of the Prism executable that was run.
	PrismExe string
	// StartedAt is when Play was pressed.
	StartedAt time.Time
	// Before is the game log as it was before Prism was started.
	Before GameLogSnapshot
}

// GameTracker follows each launched chapter from Play to the game's end: the
// game log says how far the start has got, and the game's process says when it
// ends. It reads neither Prism's account data nor any line of the log beyond
// matching it (docs/adr/0002-prism-data-root.md, third amendment).
type GameTracker struct {
	dataDir string
	emit    func(models.GameState)
	now     func() time.Time
	os      gameOS

	// tick is how often a run looks at the log; procInterval how often it
	// looks for the game's process. The timeouts are fields so tests run in
	// milliseconds.
	tick, procInterval, procSlowInterval   time.Duration
	startTimeout, prismGrace, quietTimeout time.Duration
	prismGone                              time.Duration

	mu     sync.Mutex
	states map[string]models.GameState

	timesMu sync.Mutex
}

// NewGameTracker follows games on the real OS, handing every phase change to
// emit and keeping its launch timings under dataDir.
func NewGameTracker(dataDir string, emit func(models.GameState)) *GameTracker {
	return &GameTracker{
		dataDir:          dataDir,
		emit:             emit,
		now:              time.Now,
		os:               systemGameOS(),
		tick:             logPollInterval,
		procInterval:     gameProcInterval,
		procSlowInterval: gameProcSlowInterval,
		prismGone:        gamePrismGone,
		startTimeout:     gameStartTimeout,
		prismGrace:       gamePrismGrace,
		quietTimeout:     gameLogQuiet,
		states:           map[string]models.GameState{},
	}
}

// GamePhaseActive is whether a phase is one where the game is starting,
// running or stopping: Play is held, and the instance is not to be written.
func GamePhaseActive(phase string) bool {
	return phaseRank[phase] > 0
}

// Latest returns a chapter's current state, idle when it was never launched.
func (t *GameTracker) Latest(chapterID string) models.GameState {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.states[chapterID]; ok {
		return s
	}
	return models.GameState{ChapterID: chapterID, Phase: models.GamePhaseIdle}
}

// All returns every state held, by chapter id.
func (t *GameTracker) All() []models.GameState {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]models.GameState, 0, len(t.states))
	for _, s := range t.states {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ChapterID < out[j].ChapterID })
	return out
}

// Active is whether the chapter's game is starting, running or stopping.
func (t *GameTracker) Active(chapterID string) bool {
	return GamePhaseActive(t.Latest(chapterID).Phase)
}

// Track starts following a launch on its own goroutine, which ends with the
// game, or with ctx. A chapter already starting or running is refused.
func (t *GameTracker) Track(ctx context.Context, req TrackRequest) error {
	r, err := t.begin(ctx, req)
	if err != nil {
		return err
	}
	go r.loop()
	return nil
}

// begin records the launch as starting and returns the run that follows it,
// which the caller steps: Track on a goroutine, a test by hand.
func (t *GameTracker) begin(ctx context.Context, req TrackRequest) (*gameRun, error) {
	started := req.StartedAt.UTC()
	state := models.GameState{
		ChapterID: req.ChapterID,
		Phase:     models.GamePhaseStarting,
		Since:     started.Format(time.RFC3339),
		StartedAt: started.Format(time.RFC3339),
	}
	t.mu.Lock()
	if GamePhaseActive(t.states[req.ChapterID].Phase) {
		t.mu.Unlock()
		return nil, fmt.Errorf("%s is already starting or running", req.ChapterID)
	}
	t.states[req.ChapterID] = state
	t.mu.Unlock()
	t.publish(state)

	return &gameRun{
		t:        t,
		ctx:      ctx,
		req:      req,
		state:    state,
		follower: newLogFollower(req.InstanceDir, req.Before, req.StartedAt),
		reached:  map[string]time.Time{},
		ignored:  map[int]bool{},
		exitCh:   make(chan procExit, 1),
	}, nil
}

// Durations returns the timings of a chapter's last starts that reached the
// running phase, oldest first, for the splash's estimate (#43).
func (t *GameTracker) Durations(chapterID string) []models.LaunchTiming {
	t.timesMu.Lock()
	defer t.timesMu.Unlock()
	return t.loadTimes()[chapterID]
}

func (t *GameTracker) publish(s models.GameState) {
	slog.Info("game phase", "chapter", s.ChapterID, "phase", s.Phase)
	if t.emit != nil {
		t.emit(s)
	}
}

func (t *GameTracker) loadTimes() map[string][]models.LaunchTiming {
	times := map[string][]models.LaunchTiming{}
	if t.dataDir == "" {
		return times
	}
	raw, err := os.ReadFile(filepath.Join(t.dataDir, launchTimesFile))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("launch times read", "error", err)
		}
		return times
	}
	if err := json.Unmarshal(raw, &times); err != nil {
		// A file that cannot be read is replaced on the next write: it holds
		// estimates, nothing the player wrote.
		slog.Warn("launch times parse", "error", err)
		return map[string][]models.LaunchTiming{}
	}
	return times
}

// recordTiming appends one start's timings for a chapter, keeping the last
// launchTimesKept.
func (t *GameTracker) recordTiming(chapterID string, timing models.LaunchTiming) {
	if t.dataDir == "" {
		return
	}
	t.timesMu.Lock()
	defer t.timesMu.Unlock()
	times := t.loadTimes()
	kept := append(times[chapterID], timing)
	if len(kept) > launchTimesKept {
		kept = kept[len(kept)-launchTimesKept:]
	}
	times[chapterID] = kept
	raw, err := json.MarshalIndent(times, "", "  ")
	if err != nil {
		slog.Warn("launch times encode", "error", err)
		return
	}
	if err := writeFileAtomic(filepath.Join(t.dataDir, launchTimesFile), raw, 0o600); err != nil {
		slog.Warn("launch times write", "error", err)
	}
}

// procExit is how a waited-on process ended.
type procExit struct {
	code  int
	known bool
	err   error
}

// gameRun is one launch being followed. Everything but exitCh is touched by
// its own goroutine alone.
type gameRun struct {
	t        *GameTracker
	ctx      context.Context
	req      TrackRequest
	state    models.GameState
	follower *logFollower
	// reached is when each of the timed phases was first seen.
	reached map[string]time.Time
	// stopped is whether the log has shown "Stopping!".
	stopped bool

	prismExitedAt time.Time
	// prismExitedFresh is whether the game log had begun when Prism exited.
	prismExitedFresh bool
	procUnavailable  bool
	lastProcPoll     time.Time
	// bound is the pid of the game's Java being waited on, 0 when none.
	bound      int
	ignored    map[int]bool
	exitCh     chan procExit
	cancelWait context.CancelFunc
}

func (r *gameRun) loop() {
	defer func() {
		if r.cancelWait != nil {
			r.cancelWait()
		}
	}()
	ticker := time.NewTicker(r.t.tick)
	defer ticker.Stop()
	for {
		if r.step(r.t.now()) {
			return
		}
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// step looks at the log, at Prism and at the game once, and reports whether
// the run has ended.
func (r *gameRun) step(now time.Time) bool {
	if r.prismExitedAt.IsZero() {
		select {
		case <-r.req.Prism.Exited:
			r.prismExitedAt = now
			// Whether the game log had begun before Prism went, as of the
			// last look: a Prism that goes first has handed the launch to
			// another one, and one that goes after has quit with the game.
			r.prismExitedFresh = r.follower.Fresh()
		default:
		}
	}
	r.readLog(now)
	select {
	case ex := <-r.exitCh:
		if r.gameExited(ex, now) {
			return true
		}
	default:
	}
	if r.bound == 0 && !r.procUnavailable && now.Sub(r.lastProcPoll) >= r.procInterval() {
		r.lastProcPoll = now
		r.findGame()
	}
	return r.timedOut(now)
}

// procInterval is how long to leave between looks for the game's process: the
// quick one until the loader is reading mods, the slow one after, for a log
// that got ahead of the lookup.
func (r *gameRun) procInterval() time.Duration {
	if r.state.Phase == models.GamePhaseStarting || r.state.Phase == models.GamePhaseMods {
		return r.t.procInterval
	}
	return r.t.procSlowInterval
}

func (r *gameRun) readLog(now time.Time) {
	phases, restarted := r.follower.poll(now)
	if restarted {
		// A new run wrote over the log: what was learned of the last one is
		// void, and the phases begin again.
		r.stopped = false
		clear(r.reached)
		r.set(models.GamePhaseStarting, now, nil)
	}
	for _, phase := range phases {
		r.advance(phase, now)
	}
}

// advance moves to a phase the log showed, if it is ahead of this one.
func (r *gameRun) advance(phase string, now time.Time) {
	if phaseRank[phase] <= phaseRank[r.state.Phase] {
		return
	}
	switch phase {
	case models.GamePhaseMods, models.GamePhaseWindow, models.GamePhaseResources, models.GamePhaseRunning:
		r.reached[phase] = now
	case models.GamePhaseStopping:
		r.stopped = true
	}
	r.set(phase, now, nil)
	if phase == models.GamePhaseRunning {
		r.t.recordTiming(r.req.ChapterID, r.timing())
	}
}

func (r *gameRun) timing() models.LaunchTiming {
	ms := map[string]int64{}
	for phase, at := range r.reached {
		ms[phase] = at.Sub(r.req.StartedAt).Milliseconds()
	}
	return models.LaunchTiming{StartedAt: r.req.StartedAt.UTC().Format(time.RFC3339), PhaseMs: ms}
}

func (r *gameRun) set(phase string, now time.Time, code *int) {
	r.state.Phase = phase
	r.state.Since = now.UTC().Format(time.RFC3339)
	r.state.ExitCode = code
	r.t.mu.Lock()
	r.t.states[r.req.ChapterID] = r.state
	r.t.mu.Unlock()
	r.t.publish(r.state)
}

// gameExited handles the waited-on process ending, and reports whether the
// run is over. A Java that ends before any game log began was not the game
// (Prism's pre-launch command runs one, for packwiz) or never got as far as
// one: it is set aside and the search goes on.
func (r *gameRun) gameExited(ex procExit, now time.Time) bool {
	pid := r.bound
	r.bound = 0
	if r.cancelWait != nil {
		r.cancelWait()
	}
	if ex.err != nil {
		if errors.Is(ex.err, context.Canceled) {
			return true
		}
		slog.Info("game process wait", "chapter", r.req.ChapterID, "error", ex.err)
		r.ignored[pid] = true
		return false
	}
	// The log is written before the process exits, so read it once more.
	r.readLog(now)
	if !r.follower.Fresh() {
		r.ignored[pid] = true
		return false
	}
	var code *int
	if ex.known {
		code = &ex.code
	}
	if r.stopped {
		r.set(models.GamePhaseClosed, now, code)
	} else {
		r.set(models.GamePhaseCrashed, now, code)
	}
	return true
}

// timedOut ends the run when nothing more can be expected of it.
func (r *gameRun) timedOut(now time.Time) bool {
	waiting := r.state.Phase == models.GamePhaseStarting && !r.follower.Fresh()
	switch {
	case waiting && now.Sub(r.req.StartedAt) >= r.t.startTimeout,
		waiting && r.bound == 0 && !r.prismExitedAt.IsZero() && now.Sub(r.prismExitedAt) >= r.t.prismGrace:
		r.set(models.GamePhaseFailed, now, nil)
		return true
	case r.bound == 0 && r.prismExitedFresh && now.Sub(r.prismExitedAt) >= r.t.prismGone:
		// The launcher's Prism quit after the game began, and no game process
		// is being waited on: the game went with it. One more look at the log
		// first, as it is when a waited-on process ends.
		r.readLog(now)
		if r.stopped {
			r.set(models.GamePhaseClosed, now, nil)
		} else {
			r.set(models.GamePhaseCrashed, now, nil)
		}
		return true
	case r.bound == 0 && r.stopped && r.state.Phase == models.GamePhaseStopping &&
		now.Sub(r.follower.LastGrowth()) >= r.t.quietTimeout:
		// No process to wait on, so the log alone says the game is over.
		r.set(models.GamePhaseClosed, now, nil)
		return true
	}
	return false
}

// findGame looks for the game's Java among Prism's children and, finding it,
// waits on it in the background.
func (r *gameRun) findGame() {
	procs, err := r.t.os.list()
	if err != nil {
		slog.Info("game process lookup unavailable, following the log only", "chapter", r.req.ChapterID, "error", err)
		r.procUnavailable = true
		return
	}
	parents := []int{r.req.Prism.PID}
	if !r.prismExitedAt.IsZero() {
		// [verify] Play while Prism was already open: the launcher's Prism
		// handed the launch to that one and exited, and the game's Java is
		// the other Prism's child. Not yet seen on a real install (#44).
		for _, p := range procs {
			if p.PID != r.req.Prism.PID && sameExe(p.Name, r.req.PrismExe) {
				parents = append(parents, p.PID)
			}
		}
	}
	best, bestStart := 0, time.Time{}
	for _, p := range procs {
		if !isJavaName(p.Name) || !slices.Contains(parents, p.PPID) || r.ignored[p.PID] {
			continue
		}
		created, known := r.t.os.started(p.PID)
		switch {
		case known && created.Before(r.req.StartedAt):
			continue
		case !known && p.PPID != r.req.Prism.PID:
			continue
		}
		// Of several, the newest: a pre-launch Java comes before the game.
		if best == 0 || created.After(bestStart) {
			best, bestStart = p.PID, created
		}
	}
	if best == 0 {
		return
	}
	r.bound = best
	wctx, cancel := context.WithCancel(r.ctx)
	r.cancelWait = cancel
	go func() {
		code, known, err := r.t.os.wait(wctx, best)
		r.exitCh <- procExit{code: code, known: known, err: err}
	}()
}

// isJavaName is whether a process name is a Java runtime: javaw.exe or
// java.exe on Windows, java on macOS. All three are accepted everywhere, as no
// one OS has more than one of them.
func isJavaName(name string) bool {
	switch strings.ToLower(name) {
	case "javaw.exe", "java.exe", "java":
		return true
	}
	return false
}

// sameExe compares a process name with the Prism executable's base name,
// ignoring case and a ".exe" either side lacks.
func sameExe(name, exe string) bool {
	trim := func(s string) string { return strings.TrimSuffix(strings.ToLower(s), ".exe") }
	return exe != "" && trim(name) == trim(filepath.Base(exe))
}
