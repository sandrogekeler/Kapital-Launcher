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
	"sync/atomic"
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
	// HoldWindow keeps the game's window hidden from its creation until the
	// resource reload begins, when it is shown and given the foreground
	// (#45), so the loading splash (#43) is what the player sees meanwhile.
	// True when the splash is on for this run.
	HoldWindow bool
	// OnHandover is called once, when the handover is done: the resource
	// reload has begun (or the game is running, if the log skipped that) and
	// the held window, if there was one, has been shown and given the
	// foreground. The splash minimises the launcher here, and not before,
	// because minimising first hands the foreground elsewhere. It runs on the
	// tracker's own goroutine or one it starts, and may be nil.
	OnHandover func()
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
	// hold starts keeping a game's window hidden (#45); injected so a fake
	// answers in tests.
	hold func(pid int) (WindowHolder, error)
	// holdWarned is whether a failure to hold has been logged: once is enough.
	holdWarned atomic.Bool

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
		hold:             HoldGameWindow,
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
		// Taken before this run adds its own timings, so every event of the
		// run carries the same one.
		Estimate: LaunchEstimate(t.Durations(req.ChapterID)),
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

// LaunchEstimate is the mean time from Play to each phase over earlier starts,
// in milliseconds: each of "mods", "window", "resources" and "running" is
// averaged over the starts that have it, and one no start has is left out. No
// starts, no estimate (nil).
func LaunchEstimate(times []models.LaunchTiming) map[string]int64 {
	var sum, n [4]int64
	phases := [4]string{models.GamePhaseMods, models.GamePhaseWindow, models.GamePhaseResources, models.GamePhaseRunning}
	for _, timing := range times {
		for i, phase := range phases {
			if ms, ok := timing.PhaseMs[phase]; ok {
				sum[i] += ms
				n[i]++
			}
		}
	}
	var est map[string]int64
	for i, phase := range phases {
		if n[i] == 0 {
			continue
		}
		if est == nil {
			est = map[string]int64{}
		}
		// Rounded to the nearest millisecond.
		est[phase] = (sum[i] + n[i]/2) / n[i]
	}
	return est
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
	bound int
	// holder keeps the bound game's window hidden, nil when none is held.
	holder WindowHolder
	// handedOver is whether the handover has begun, so it happens once.
	handedOver bool
	ignored    map[int]bool
	exitCh     chan procExit
	cancelWait context.CancelFunc
}

func (r *gameRun) loop() {
	defer func() {
		if r.cancelWait != nil {
			r.cancelWait()
		}
		// However the run ends, a window that is still alive must not stay
		// hidden.
		r.releaseWindow(false)
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
	// The handover is the resource reload beginning, or the game running when
	// the log skipped that; whichever comes first finds the window held (#45).
	switch phase {
	case models.GamePhaseResources, models.GamePhaseRunning:
		r.handover()
	case models.GamePhaseStopping, models.GamePhaseClosed, models.GamePhaseCrashed, models.GamePhaseFailed:
		r.releaseWindow(false)
	}
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
	// The process is gone, and so is its window. A Java set aside below was
	// not the game, and the next one is held afresh.
	r.releaseWindow(false)
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
	r.holdWindow(best)
	wctx, cancel := context.WithCancel(r.ctx)
	r.cancelWait = cancel
	go func() {
		code, known, err := r.t.os.wait(wctx, best)
		r.exitCh <- procExit{code: code, known: known, err: err}
	}()
}

// holdWindow starts keeping the game's window hidden, when asked to and while
// the handover has not come. It never fails the run: where it cannot hold, the
// window shows as the game makes it.
func (r *gameRun) holdWindow(pid int) {
	if !r.req.HoldWindow || r.t.hold == nil || r.holder != nil ||
		phaseRank[r.state.Phase] >= phaseRank[models.GamePhaseResources] {
		return
	}
	h, err := r.t.hold(pid)
	if err != nil {
		if r.t.holdWarned.CompareAndSwap(false, true) {
			slog.Warn("game window cannot be held", "chapter", r.req.ChapterID, "error", err)
		}
		return
	}
	r.holder = h
}

// handover is the game having the screen: the held window, if there is one,
// is shown and given the foreground, and then OnHandover is called, once per
// run. The foreground release nudges the window and so takes a few hundred ms:
// it runs on its own goroutine with the callback after it, and the run loop
// goes on. With no window held there is nothing to wait for and the callback
// runs here.
func (r *gameRun) handover() {
	if r.handedOver {
		return
	}
	r.handedOver = true
	h := r.holder
	r.holder = nil
	done := r.req.OnHandover
	if h == nil {
		if done != nil {
			done()
		}
		return
	}
	go func() {
		r.logRelease(true, h.Release(true))
		if done != nil {
			done()
		}
	}()
}

// releaseWindow shows the held window again without the foreground and logs
// what holding it came to. It is quick and done before the run moves on, so a
// window is never left hidden behind a run that ended.
func (r *gameRun) releaseWindow(foreground bool) {
	if r.holder == nil {
		return
	}
	h := r.holder
	r.holder = nil
	r.logRelease(foreground, h.Release(foreground))
}

// logRelease records what holding a window came to: counts and milliseconds,
// never a window title.
func (r *gameRun) logRelease(handover bool, rep WindowReport) {
	slog.Info("game window", "chapter", r.req.ChapterID, "handover", handover,
		"seen", rep.Seen, "swept", rep.Swept, "hides", rep.Hides,
		"firstHideMs", rep.FirstHideMs, "maxHideMs", rep.MaxHideMs,
		"foreground", rep.Foreground, "nudged", rep.Nudged)
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
