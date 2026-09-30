package models

// Phases of a launched chapter's game, from Play to its end. The first seven
// follow the game's own start; the last three are how it ended. Emitted in
// GameState as the "game:state" event.
const (
	// GamePhaseIdle is a chapter that was not launched by this app run.
	GamePhaseIdle = "idle"
	// GamePhaseStarting is Play pressed, with no fresh game log yet.
	GamePhaseStarting = "starting"
	// GamePhaseMods is a fresh log having begun: the loader is reading mods.
	GamePhaseMods = "mods"
	// GamePhaseWindow is the game's window being opened (LWJGL is up).
	GamePhaseWindow = "window"
	// GamePhaseResources is the resource reload, textures and sounds.
	GamePhaseResources = "resources"
	// GamePhaseRunning is the main menu being ready.
	GamePhaseRunning = "running"
	// GamePhaseStopping is the game having begun to close.
	GamePhaseStopping = "stopping"
	// GamePhaseClosed is the game having exited after stopping.
	GamePhaseClosed = "closed"
	// GamePhaseCrashed is the game having exited without stopping.
	GamePhaseCrashed = "crashed"
	// GamePhaseFailed is no game ever having appeared.
	GamePhaseFailed = "failed"
)

// GameState is where a launched chapter's game is, from Play to its end.
// Keyed by the chapter, so a listener filters on ChapterID.
type GameState struct {
	ChapterID string `json:"chapterId"`
	// Phase is one of the GamePhase constants.
	Phase string `json:"phase"`
	// Since is when this phase began, RFC 3339 in UTC.
	Since string `json:"since"`
	// StartedAt is when Play was pressed, RFC 3339 in UTC. Empty for idle.
	StartedAt string `json:"startedAt"`
	// ExitCode is the game's own exit code, when the platform reports it.
	ExitCode *int `json:"exitCode,omitempty"`
}

// LaunchTiming is how long one start took to reach each phase, in
// milliseconds from Play. A phase the log never showed is absent.
type LaunchTiming struct {
	// StartedAt is when Play was pressed, RFC 3339 in UTC.
	StartedAt string `json:"startedAt"`
	// PhaseMs maps "mods", "window", "resources" and "running" to the
	// milliseconds from Play to that phase.
	PhaseMs map[string]int64 `json:"phaseMs"`
}
