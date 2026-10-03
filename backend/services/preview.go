package services

import (
	"maps"
	"strings"
	"sync"
	"time"

	"kapital/backend/models"
)

// Developer previews (#124): the screens a run only shows when something goes
// wrong, faked on demand. Everything here is state and copy. The constructors
// are pure: each returns the value the real path would have produced (a
// GameState, a RunReport, a PackState, an EngineInfo, a PrismRelease, the
// prism:install steps) and reads no file, starts no process and reaches no
// network. A log tail is a constant in this file, never read from a disk.

// The situations StartPreview takes. A chapter's, then Prism's.
const (
	PreviewStartFailedSync  = "start-failed-sync"
	PreviewStartFailedPrism = "start-failed-prism"
	PreviewCrashed          = "crashed"
	PreviewStopped          = "stopped"
	PreviewRunning          = "running"
	PreviewStarting         = "starting"
	PreviewConsole          = "console"
	PreviewNotInstalled     = "not-installed"
	PreviewSourceAhead      = "source-ahead"

	PreviewPrismMissing = "prism-missing"
	PreviewPrismInstall = "prism-install"
	PreviewPrismUpdate  = "prism-update"
)

// previewSituations is the fixed list, in the order the buttons show.
var previewSituations = []models.PreviewSituation{
	{ID: PreviewStartFailedSync, Label: "Start failed: pack sync", Scope: models.PreviewScopeChapter, Card: true},
	{ID: PreviewStartFailedPrism, Label: "Start failed: Prism stopped before the game", Scope: models.PreviewScopeChapter, Card: true},
	{ID: PreviewCrashed, Label: "Crashed, with a crash report", Scope: models.PreviewScopeChapter, Card: true},
	{ID: PreviewConsole, Label: "Start failed, Prism's console on the card", Scope: models.PreviewScopeChapter, Card: true},
	{ID: PreviewStarting, Label: "Starting, on the loading card", Scope: models.PreviewScopeChapter, Card: true},
	{ID: PreviewRunning, Label: "Running, to try Stop", Scope: models.PreviewScopeChapter},
	{ID: PreviewStopped, Label: "Stopped by the player", Scope: models.PreviewScopeChapter},
	{ID: PreviewNotInstalled, Label: "Not installed", Scope: models.PreviewScopeChapter},
	{ID: PreviewSourceAhead, Label: "Pack source is ahead", Scope: models.PreviewScopeChapter},
	{ID: PreviewPrismMissing, Label: "Prism missing", Scope: models.PreviewScopePrism},
	{ID: PreviewPrismInstall, Label: "Prism install, then it fails", Scope: models.PreviewScopePrism, PlaysInstall: true},
	{ID: PreviewPrismUpdate, Label: "Prism update available", Scope: models.PreviewScopePrism},
}

// PreviewSituations is the list of situations, for the frontend to render.
func PreviewSituations() []models.PreviewSituation {
	return append([]models.PreviewSituation(nil), previewSituations...)
}

// PreviewSituationByID finds a situation by the name StartPreview was given.
func PreviewSituationByID(id string) (models.PreviewSituation, bool) {
	for _, s := range previewSituations {
		if s.ID == id {
			return s, true
		}
	}
	return models.PreviewSituation{}, false
}

// previewCrashReport is the crash report's name a crashed preview shows: a
// file name that is plainly not a real one.
const previewCrashReport = "crash-preview-client.txt"

// previewLogTail is the end of a made-up game log. Every line says it is one.
const previewLogTail = `[preview] This is a made-up log. No game ran and no file was read.
[12:00:00] [main/INFO] [preview]: Loading 3 made-up mods
[12:00:09] [main/INFO] [preview]: Made-up mod one is ready
[12:00:17] [main/INFO] [preview]: Made-up mod two is ready
[12:00:31] [Render thread/INFO] [preview]: Reloading made-up resources
[12:00:52] [Render thread/ERROR] [preview]: Something made up went wrong
[12:00:52] [Render thread/FATAL] [preview]: Reported made-up exception, saving it as ` + previewCrashReport + `
`

// previewEstimate is the start's estimate on a previewed run, in milliseconds
// from Play: what the card's bar eases along.
var previewEstimate = map[string]int64{"mods": 6000, "window": 24000, "resources": 31000, "running": 52000}

// previewPhases is when a previewed run that got as far as the menu reached each phase.
var previewPhases = []models.PhaseTime{
	{Phase: models.GamePhaseStarting, Ms: 0},
	{Phase: models.GamePhaseMods, Ms: 6000},
	{Phase: models.GamePhaseWindow, Ms: 24000},
	{Phase: models.GamePhaseResources, Ms: 31000},
	{Phase: models.GamePhaseRunning, Ms: 52000},
}

// previewRun is how one situation's run looks: the phase it is in, how long ago
// Play was pressed and the phase began, and why it ended.
type previewRun struct {
	phase    string
	reason   string
	exitCode *int
	// ago is how long before now Play was pressed, sinceAgo the phase began.
	ago, sinceAgo time.Duration
	// phases are how far the run got.
	phases []models.PhaseTime
	// log is whether a game log exists to show the end of.
	log, crashReport, console bool
}

var previewExitCode = 1

// previewRuns is the run behind each situation that has one.
var previewRuns = map[string]previewRun{
	PreviewStartFailedSync:  {phase: models.GamePhaseFailed, reason: models.GameFailPackSync, ago: 9 * time.Second, phases: previewPhases[:1]},
	PreviewStartFailedPrism: {phase: models.GamePhaseFailed, reason: models.GameFailLaunch, ago: 6 * time.Second, phases: previewPhases[:1]},
	PreviewConsole:          {phase: models.GamePhaseFailed, reason: models.GameFailLaunch, ago: 7 * time.Second, phases: previewPhases[:1], console: true},
	PreviewCrashed: {
		phase: models.GamePhaseCrashed, exitCode: &previewExitCode, ago: 2 * time.Minute, phases: previewPhases,
		log: true, crashReport: true,
	},
	PreviewStopped:  {phase: models.GamePhaseCrashed, reason: models.GameFailStopped, ago: time.Minute, phases: previewPhases},
	PreviewRunning:  {phase: models.GamePhaseRunning, ago: 90 * time.Second, sinceAgo: 30 * time.Second, phases: previewPhases},
	PreviewStarting: {phase: models.GamePhaseMods, ago: 20 * time.Second, sinceAgo: 14 * time.Second, phases: previewPhases[:2]},
}

// PreviewGameState is the synthetic game:state of a situation that has a run
// (the card's, the bar's and the notices'), or false for one that has none.
// Since is now, so a notice the player dismissed before comes back.
func PreviewGameState(situation, chapterID string, now time.Time) (models.GameState, bool) {
	r, ok := previewRuns[situation]
	if !ok {
		return models.GameState{}, false
	}
	stamp := func(ago time.Duration) string { return now.Add(-ago).UTC().Format(time.RFC3339) }
	state := models.GameState{
		ChapterID: chapterID,
		Phase:     r.phase,
		Since:     stamp(r.sinceAgo),
		StartedAt: stamp(r.ago),
		Reason:    r.reason,
		Estimate:  maps.Clone(previewEstimate),
	}
	if r.exitCode != nil {
		code := *r.exitCode
		state.ExitCode = &code
	}
	return state, true
}

// PreviewRunReport is the synthetic report of a situation that has a run: its
// phases, a made-up log tail where a game would have written one, a crash
// report's name for a crash, and whether Prism's console is on offer.
func PreviewRunReport(situation, chapterID string, now time.Time) (models.RunReport, bool) {
	r, ok := previewRuns[situation]
	if !ok {
		return models.RunReport{}, false
	}
	game, _ := PreviewGameState(situation, chapterID, now)
	report := models.RunReport{
		Game:             game,
		Phases:           append([]models.PhaseTime(nil), r.phases...),
		ConsoleAvailable: r.console,
	}
	if r.log {
		report.LogTail = previewLogTail
		report.LogLines = strings.Count(previewLogTail, "\n")
	}
	if r.crashReport {
		report.CrashReport = previewCrashReport
	}
	return report, true
}

// PreviewPackState is the synthetic pack state of a chapter's situation, or
// false for one that does not touch it. The source ahead is a pack installed
// and checked that is not the source's, with a version named.
func PreviewPackState(situation, chapterID string) (models.PackState, bool) {
	switch situation {
	case PreviewSourceAhead:
		return models.PackState{ChapterID: chapterID, Installed: true, Checked: true, UpToDate: false, Version: "9.9.9"}, true
	case PreviewNotInstalled:
		return models.PackState{ChapterID: chapterID}, true
	}
	return models.PackState{}, false
}

// PreviewInstances is the report with the chapter's instance faked to the
// situation: missing for not installed, present for the source being ahead.
// The report it is given is not changed.
func PreviewInstances(report models.InstanceReport, situation, chapterID string) models.InstanceReport {
	if situation != PreviewNotInstalled && situation != PreviewSourceAhead {
		return report
	}
	out := report
	out.Present = maps.Clone(report.Present)
	if out.Present == nil {
		out.Present = map[string]bool{}
	}
	if situation == PreviewSourceAhead {
		out.Present[chapterID] = true
		return out
	}
	out.Present[chapterID] = false
	out.PackURL = maps.Clone(report.PackURL)
	delete(out.PackURL, chapterID)
	out.SizeBytes = maps.Clone(report.SizeBytes)
	delete(out.SizeBytes, chapterID)
	return out
}

// PreviewEngine is the synthetic engine of a Prism situation, or false for a
// situation that is not Prism's. Missing and the install have no Prism; the
// update is the launcher's own copy, which is the one it offers to update.
func PreviewEngine(situation string) (models.EngineInfo, bool) {
	switch situation {
	case PreviewPrismMissing, PreviewPrismInstall:
		return models.EngineInfo{}, true
	case PreviewPrismUpdate:
		return models.EngineInfo{Found: true, Version: previewPrismInstalled, Source: "managed"}, true
	}
	return models.EngineInfo{}, false
}

const (
	previewPrismVersion   = "9.9.9"
	previewPrismInstalled = "9.0.0"
	previewPrismSize      = 20 << 20
)

// PreviewRelease is the synthetic Prism release of a Prism situation: a named
// version, with an update available for the update's. Its asset address is
// never fetched: nothing under a preview downloads.
func PreviewRelease(situation string) (models.PrismRelease, bool) {
	if _, ok := PreviewEngine(situation); !ok {
		return models.PrismRelease{}, false
	}
	rel := models.PrismRelease{
		Version: previewPrismVersion,
		Asset:   "PrismLauncher-preview.zip",
		URL:     "https://github.com/PrismLauncher/PrismLauncher/releases/download/" + previewPrismVersion + "/PrismLauncher-preview.zip",
		Size:    previewPrismSize,
		Digest:  "sha256:" + strings.Repeat("0", 64),
		Page:    "https://github.com/PrismLauncher/PrismLauncher/releases",
	}
	if situation == PreviewPrismUpdate {
		rel.Installed = previewPrismInstalled
		rel.UpdateAvailable = true
	}
	return rel, true
}

// previewInstallError is what the made-up install fails with.
const previewInstallError = "The download failed (preview: nothing was downloaded)"

// PreviewInstallProgress is the prism:install steps a preview plays: a download
// that fills, the check of it, and then the failure, which is the last step.
func PreviewInstallProgress() []models.PrismInstallProgress {
	const total = previewPrismSize
	steps := []models.PrismInstallProgress{
		{Phase: "downloading", Received: 0, Total: total},
		{Phase: "downloading", Received: total * 35 / 100, Total: total},
		{Phase: "downloading", Received: total * 70 / 100, Total: total},
		{Phase: "downloading", Received: total, Total: total},
		{Phase: "verifying", Received: total, Total: total},
		{Phase: "failed", Error: previewInstallError},
	}
	return steps
}

// previewEntry is a chapter's preview: the situation and the moment it began,
// which its synthetic state is dated from, so the state is the same every time
// it is asked for.
type previewEntry struct {
	situation string
	at        time.Time
}

// PreviewSet holds the previews that are on: per chapter at most one
// situation, and at most one of Prism's. It holds names and a time and nothing
// else; the zero value is ready, and nothing is persisted, so a restart clears it.
type PreviewSet struct {
	mu       sync.Mutex
	chapters map[string]previewEntry
	prism    string
}

// SetChapter makes the situation the chapter's preview, begun at, replacing its last.
func (p *PreviewSet) SetChapter(chapterID, situation string, at time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.chapters == nil {
		p.chapters = map[string]previewEntry{}
	}
	p.chapters[chapterID] = previewEntry{situation: situation, at: at}
}

// Chapter is the chapter's situation, or "" when it has none.
func (p *PreviewSet) Chapter(chapterID string) string {
	situation, _ := p.ChapterAt(chapterID)
	return situation
}

// ChapterAt is the chapter's situation and when it began; "" for none.
func (p *PreviewSet) ChapterAt(chapterID string) (string, time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e := p.chapters[chapterID]
	return e.situation, e.at
}

// ClearChapter ends the chapter's preview and says which it was, "" for none.
func (p *PreviewSet) ClearChapter(chapterID string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	was := p.chapters[chapterID].situation
	delete(p.chapters, chapterID)
	return was
}

// Chapters is every chapter's situation now, by chapter id.
func (p *PreviewSet) Chapters() map[string]string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make(map[string]string, len(p.chapters))
	for id, e := range p.chapters {
		out[id] = e.situation
	}
	return out
}

// SetPrism makes the situation Prism's preview, replacing its last.
func (p *PreviewSet) SetPrism(situation string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.prism = situation
}

// Prism is Prism's situation, or "" when it has none.
func (p *PreviewSet) Prism() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.prism
}

// ClearPrism ends Prism's preview and says which it was, "" for none.
func (p *PreviewSet) ClearPrism() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	was := p.prism
	p.prism = ""
	return was
}
