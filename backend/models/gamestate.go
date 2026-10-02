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

// Why a start failed, in GameState.Reason. A failure that is neither carries
// no reason: the start timeout, and a Prism that went with no game.
const (
	// GameFailPackSync is Prism's pre-launch command failing, which is the
	// pack sync (packwiz-installer): the pack server was down or unreachable.
	GameFailPackSync = "packsync"
	// GameFailLaunch is any other launch step Prism stopped at: Java, the
	// libraries, the assets.
	GameFailLaunch = "launch"
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
	// Reason is why a failed start failed, one of the GameFail constants, read
	// from Prism's launcher log (#103). Empty for every other phase and for a
	// failure whose cause is not known.
	Reason string `json:"reason,omitempty"`
	// Splash is whether this run shows the loading card and the player has not
	// left it (#43). Set by the App on the way out, never by the tracker.
	Splash bool `json:"splash,omitempty"`
	// Estimate is how long this run is expected to take to reach each of
	// "mods", "window", "resources" and "running", in milliseconds from Play:
	// the mean of the chapter's earlier starts. A phase none of them reached is
	// absent, and a chapter with no earlier start has no estimate. The same on
	// every event of the run, so the card's bar needs nothing else.
	Estimate map[string]int64 `json:"estimate,omitempty"`
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
