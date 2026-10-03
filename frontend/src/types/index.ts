/**
 * The app's data shapes. They are Go structs in backend/models/, and Wails
 * generates classes for them in wailsjs/go/models.ts; those classes carry
 * constructor helpers a plain JSON literal cannot satisfy, so src/ types
 * against these interfaces instead. Keep them field-for-field with the Go
 * structs: `pnpm typecheck` fails where a bound call disagrees.
 */

export type ChapterState = 'released' | 'development' | 'planned'
export type Theme = 'dark' | 'light' | 'system'

export interface Manifest {
  $schema?: string
  version: number
  wiki: { baseUrl: string }
  chapters: Chapter[]
}

export interface Chapter {
  id: string
  number: string
  name: string
  era: string
  kind: string
  blurb: string
  state: ChapterState | string
  instance: { id: string }
  pack: Pack
  server?: Server | null
  wiki: WikiTeaser
  changelog: ChangelogEntry[]
}

export interface Server {
  address: string
  /** Whether Play joins this server, or the chapter is played as a pack with a server nearby. */
  joinOnLaunch: boolean
  software: string
}

/** The result of one Server List Ping, keyed by chapter. */
export interface ServerStatus {
  chapterId: string
  checked: boolean
  online: boolean
  players: number
  max: number
  version: string
  motd: string
  latencyMs: number
  checkedAt: string
}

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

// Go pointer fields arrive optional from the bindings, so they are optional here too.
export interface Pack {
  type: 'modpack' | 'client-visuals' | string
  loader: string
  minecraft: string
  mods?: number | null
  memoryGb?: number | null
  /** A JVM preset the launcher owns, e.g. 'zgc'; never raw arguments. */
  jvm?: string | null
  version?: string | null
  packwiz?: string | null
  mrpack?: string | null
}

/** One page of the wiki, from its lore export (#58); eras are chapter names. */
export interface WikiPage {
  title: string
  line: string
  url: string
  eras: string[]
  /** The page's id in the export, which a screenshot's subject names (#141). */
  id: string
  /** The ids of the listed pages the wiki relates to this one, both ways. */
  related: string[]
}

/** One of the wiki's screenshots, cached and served by Go at `src` (#141). */
export interface WikiShot {
  era: string
  src: string
  /** The id of the page the picture shows, '' when none. */
  subject: string
}

export interface WikiTeaser {
  title: string
  line: string
  path: string
}

export interface ChangelogEntry {
  version: string
  summary: string
}

export interface EngineInfo {
  found: boolean
  executable: string
  version: string
  root: string
  source: string
}

/**
 * Which chapters' Prism instances exist. A chapter missing from `present` is
 * unknown (the root could not be worked out), not absent.
 */
export interface InstanceReport {
  root: string
  dir: string
  present: Record<string, boolean>
  /** The pack URL each launcher-made instance syncs from (#41). */
  packUrl: Record<string, string>
  /** What each present instance takes on disk, in bytes (#57). */
  sizeBytes: Record<string, number>
}

/** A Prism release the launcher can install for a player without Prism (ADR-11). */
export interface PrismRelease {
  version: string
  asset: string
  url: string
  size: number
  digest: string
  page: string
  /** The launcher-managed Prism's version, "" when there is none. */
  installed: string
  /** Only ever true when the managed Prism is the one in use. */
  updateAvailable: boolean
}

export type PrismInstallPhase = 'downloading' | 'verifying' | 'unpacking' | 'done' | 'failed'

/** The prism:install event: one per step of an install or update. */
export interface PrismInstallProgress {
  phase: PrismInstallPhase | string
  received: number
  total: number
  error: string
}

/** Whether a chapter's installed pack is the one its source serves now (#71). */
export interface PackState {
  chapterId: string
  installed: boolean
  checked: boolean
  upToDate: boolean
  version: string
}

/** The two packs an installed chapter can sync from: the manifest's, or the local one from settings. */
export type PackSource = 'published' | 'dev'

/** What a player may change about a chapter's installed instance (#36). */
export interface ChapterSettings {
  maxMemoryMb: number
  /** A preset name from the launcher's list, or '' for Prism's own arguments. */
  jvm: string
}

/** A chapter's settings with what the panel shows around them. */
export interface ChapterSettingsInfo {
  chapterId: string
  settings: ChapterSettings
  /** The machine's memory in MB, 0 when unknown. */
  machineMemoryMb: number
  prismDefaultMb: number
  /** The manifest's memory for the chapter in MB, 0 when it names none. */
  packMemoryMb: number
  presets: string[]
  running: boolean
}

export interface AppSettings {
  theme: Theme | string
  prismExecutable: string
  prismRoot: string
  profileName: string
  lastChapter: string
  /** Chapter id to a local packwiz serve address (#41), edited on the settings screen (#5). */
  packOverrides?: Record<string, string>
  /** The player's choice for the loading splash (#43, #97); absent is the default: on for Windows, off for macOS, unavailable elsewhere. */
  loadingSplash?: boolean
  /** Whether the splash can run on this OS at all (Windows and macOS). Derived by GetSettings, never saved. */
  loadingSplashAvailable?: boolean
  /** Whether the next Play shows the splash: the effective value of loadingSplash on this OS. Derived, never saved. */
  loadingSplashOn?: boolean
  /** The slideshow off (issue 142): each chapter shows its own bundled picture, with no timer. Absent is on. */
  staticArt?: boolean
}
