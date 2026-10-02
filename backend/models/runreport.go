package models

// RunReportLogBytes is how much of the game's log a RunReport carries at most:
// the end of it, where the failure is. Small enough for the card to hold and
// for one binding call; the whole log is a file the player can open.
const RunReportLogBytes = 16 << 10

// PhaseTime is when a run reached one phase.
type PhaseTime struct {
	// Phase is one of "starting", "mods", "window", "resources" and "running".
	Phase string `json:"phase"`
	// Ms is the milliseconds from Play to that phase; 0 for "starting".
	Ms int64 `json:"ms"`
}

// RunReport is what the launcher knows of a chapter's latest run, now or
// ended, for the view that replaces Prism's console (ADR-2, sixth amendment).
// It is built when asked for and kept nowhere: the log tail and the crash
// report's name are read from the player's disk for this one call.
type RunReport struct {
	// Game is the run's latest state: its phase, why a start failed, and the
	// times of Play and of the phase.
	Game GameState `json:"game"`
	// Phases is when each phase was reached, in order, from Play: always
	// "starting" at 0, then those of mods, window, resources and running the
	// game's log showed. How the run ended is Game's Phase and Since, not here.
	Phases []PhaseTime `json:"phases"`
	// LogTail is the last RunReportLogBytes of the game's latest.log, from a
	// whole line, with the player's home path, user and in-game names, server
	// addresses and IP addresses masked. Empty when no game log of this run was
	// found: the start failed before the game, so there is none to show.
	LogTail string `json:"logTail"`
	// LogLines is how many lines LogTail has.
	LogLines int `json:"logLines"`
	// LogTruncated is whether the log is longer than LogTail, which then opens
	// partway through it.
	LogTruncated bool `json:"logTruncated"`
	// CrashReport is the file name, never the path, of the newest report in the
	// game's crash-reports folder written since Play. Empty when there is none.
	CrashReport string `json:"crashReport"`
	// ConsoleAvailable is whether Prism's console can be shown from the view.
	// Always false for now: the console is not hidden yet, so there is nothing
	// to show back.
	ConsoleAvailable bool `json:"consoleAvailable"`
}
