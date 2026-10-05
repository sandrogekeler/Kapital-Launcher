/**
 * A launched chapter's game as the launcher follows it: its phases, the report of a run, and
 * the game's own logs. Mirrors the Go models; re-exported from `types/index.ts`.
 */

/** Where a launched chapter's game is. Same strings as the Go GamePhase constants. */
export type GamePhase =
  | 'idle'
  | 'starting'
  | 'mods'
  | 'window'
  | 'resources'
  | 'running'
  | 'stopping'
  | 'closed'
  | 'crashed'
  | 'failed'

/**
 * Why a run failed or ended: from Prism's launcher log for a start (#103), or `stopped` when the
 * player ended it from the launcher. Same strings as the Go GameFail constants.
 */
export type GameFailReason = 'packsync' | 'launch' | 'stopped'

/** A launched chapter's game, keyed by chapter. Times are RFC 3339 in UTC. */
export interface GameState {
  chapterId: string
  phase: GamePhase
  /** When this phase began. */
  since: string
  /** When Play was pressed; empty for idle. */
  startedAt: string
  /** The game's own exit code, when the platform reports it. */
  exitCode?: number
  /** Why a start failed or a run was ended; absent when the cause is not known. */
  reason?: GameFailReason
  /** This run shows the loading card and the player has not left it (#43). */
  splash?: boolean
  /**
   * Milliseconds from Play to each of mods, window, resources and running: the mean of the
   * chapter's earlier starts. A phase none reached is absent; no history, no estimate. The same
   * on every event of a run.
   */
  estimate?: Partial<Record<'mods' | 'window' | 'resources' | 'running', number>>
}

/** When a run reached one of starting, mods, window, resources and running. */
export interface PhaseTime {
  phase: GamePhase
  /** Milliseconds from Play; 0 for starting. */
  ms: number
}

/**
 * What the launcher knows of a chapter's latest run, now or ended: the view that
 * stands in for Prism's console (ADR-2, sixth amendment). Read when asked for and
 * kept nowhere.
 */
export interface RunReport {
  /** The run's latest state: its phase, why a start failed, the times of Play and of the phase. */
  game: GameState
  /** When each phase was reached, in order. How the run ended is `game`'s phase and `since`. */
  phases: PhaseTime[]
  /** The end of the game's log, redacted; empty when the start failed before any game log. */
  logTail: string
  logLines: number
  /** Whether the log is longer than `logTail`, which then opens partway through it. */
  logTruncated: boolean
  /** The file name, never the path, of the newest crash report since Play; empty for none. */
  crashReport: string
  /** Whether Prism's console, hidden by Go (Windows), can be shown from the view. */
  consoleAvailable: boolean
}

/** The two kinds of file the logs page lists (issue 155). */
export type RunLogKind = 'log' | 'crash'

/** One game log or crash report of a chapter's instance, as the logs page lists it. */
export interface RunLog {
  kind: RunLogKind
  /** The file's base name, which `ReadRunLog` takes back; never a path. */
  name: string
  /** When the file was last written, RFC 3339 in UTC. */
  modifiedAt: string
  /** Bytes on disk; a dated log's is its compressed size. */
  size: number
  /** Whether a crash report was written during this log's run; best effort. */
  crashed: boolean
}

/** A chunk of one RunLog, whole lines, masked by Go before it left. */
export interface RunLogText {
  kind: RunLogKind
  name: string
  text: string
  /** Where `text` begins in the file; the `before` that reads the stretch preceding it. */
  offset: number
  /** The file's length, unpacked. */
  size: number
  lines: number
  /** Whether the file has more before `text`. */
  truncated: boolean
}

/**
 * What the live log follower emits as `log:live` (issue 155): whole lines
 * appended to a chapter's latest.log, masked by Go, or, with `reset`, the end of
 * a new run's file in place of what was shown.
 */
export interface LiveLogEvent {
  chapterId: string
  lines: string
  reset: boolean
}

// Go pointer fields arrive optional from the bindings, so they are optional here too.
