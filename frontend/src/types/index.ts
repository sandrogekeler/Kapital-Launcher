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
  /** The chapter's BlueMap web map (issue 161): http or https on a playit tunnel, with a port. Absent when it has none. */
  map?: string
}

/** What one reachability check of a chapter's map found (issue 161). */
export interface MapStatus {
  /** The manifest's map address, or "" when the chapter has none. */
  url: string
  /** Whether any HTTP response came back. */
  reachable: boolean
  /** Why not, in a few words: "no map", "timed out" or "no answer". */
  reason: string
}

/** One way to reach a chapter's server, under the label settings offers (issue 151). */
export interface ServerAddress {
  label: string
  address: string
}

export interface Server {
  /** Every address, the first being the default; the player picks a label in the chapter's settings. */
  addresses: ServerAddress[]
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
  /** Mods a player may switch off from the chapter's settings as a quick choice (issue 156). */
  toggles?: ModToggle[] | null
}

/**
 * A mod the manifest offers as a quick switch: a name to show and the start of its jar's
 * file name, the part before the version, so it keeps matching when the pack updates the mod.
 */
export interface ModToggle {
  name: string
  jarPrefix: string
  /** The jarPrefix of another toggle this mod needs to load; off with it, and locked while it is. */
  requires?: string | null
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

/** One of the wiki's pictures, a screenshot or a page's, cached and served by Go at `src` (issue 172). */
export interface WikiShot {
  era: string
  src: string
  /** The id of the page the picture shows, '' when none. */
  subject: string
}

/** What the settings screen needs to estimate the room the wiki pictures take (issue 172). */
export interface WikiArtStats {
  /** The mean size of the pictures in the cache, about 100 KB while it holds none. */
  avgBytes: number
  /** How many pictures each chapter can draw from, by era id (the chapter's name). */
  pools: Record<string, number>
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

/**
 * One release in a pack's own changelog file (issue 164), shaped for the panel:
 * the summary is the entry's first line, the details are all of them.
 */
export interface PackChangelogEntry {
  version: string
  /** YYYY-MM-DD. */
  date: string
  summary: string
  details: string
}

/**
 * What a chapter's pack source publishes as its changelog, newest first.
 * Unchecked is a file that could not be read; checked with no entries is a
 * pack that publishes none. Either way there are no entries, and the
 * manifest's changelog stands in.
 */
export interface PackChangelog {
  chapterId: string
  checked: boolean
  entries: PackChangelogEntry[]
}

/** One situation the Developer section can fake (#124). The list is Go's, fixed there. */
export interface PreviewSituation {
  id: string
  label: string
  /** A chapter's own screens, or Prism's. */
  scope: 'chapter' | 'prism'
  /** Whether it opens the loading card, which needs the loading splash on. */
  card: boolean
  /** Whether the view then runs the Prism install, whose progress and failure are the situation. */
  playsInstall: boolean
}

/** What StartPreview says happened. */
export interface PreviewStart {
  /** The situation wanted the loading card and it was not opened. */
  cardSkipped: boolean
}

/** The two packs an installed chapter can sync from: the manifest's, or the local one from settings. */
export type PackSource = 'published' | 'dev'

/** What a player may change about a chapter's installed instance (#36). */
export interface ChapterSettings {
  maxMemoryMb: number
  /** A preset name from the launcher's list, or '' for Prism's own arguments. */
  jvm: string
  /** The player's own Java arguments, after the preset's (issue 191); Go holds each to its rules. */
  jvmArgs: string[]
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

/** One jar in a chapter's mods folder (issue 156). */
export interface ModFile {
  /** The jar's base name, `Mod-1.2.3.jar`, whether the file on disk is that or `.jar.disabled`. */
  name: string
  /** Whether the file is switched off: Prism's own `.jar.disabled`. */
  disabled: boolean
  /** Bytes. */
  size: number
}

/** A manifest toggle resolved against the mods folder. */
export interface ModToggleState extends ModToggle {
  /** The jars it matches now; none before the pack's first sync. */
  jars: string[]
  /** True when it matches at least one jar and every one of them is switched off. */
  disabled: boolean
  /** A toggle it requires, directly or through another, is off: it cannot be turned on now. */
  blocked: boolean
}

/** A chapter's mods as the settings page shows them: the folder, and the quick toggles in it. */
export interface ChapterMods {
  chapterId: string
  mods: ModFile[]
  toggles: ModToggleState[]
  /** The tracker or the game log says the game runs; the log half is a guess, shown as a hint. */
  running: boolean
}

/** What Prism has counted for one installed chapter (issue 192): seconds played, and the last start. */
export interface PlayTime {
  chapterId: string
  totalSeconds: number
  /** Milliseconds since the epoch; 0 when the game was never started. */
  lastLaunchMs: number
}

export interface AppSettings {
  theme: Theme | string
  prismExecutable: string
  prismRoot: string
  profileName: string
  /** Play without a Microsoft account (issue 192): Prism's --offline with `offlineName` in place of --profile. */
  offline: boolean
  /** The player name passed with --offline: 3 to 16 letters, digits or _; required while `offline` is on. */
  offlineName: string
  lastChapter: string
  /** Chapter id to a local packwiz serve address (#41), edited on the settings screen (#5). */
  packOverrides?: Record<string, string>
  /** Chapter id to the label of the server address the player picked (issue 151); absent is the first. */
  serverChoices?: Record<string, string>
  /**
   * The ids of the chapters whose server Play joins (issue 163): the switch on a chapter's
   * settings page, off for every chapter until the player turns it on.
   */
  joinServers?: string[]
  /**
   * Chapter id to the jars switched off (issue 156). Go's own: GetSettings never fills it and a
   * save never changes it; the mods section reads and writes the list through GetChapterMods and
   * SetModsDisabled.
   */
  disabledMods?: Record<string, string[]>
  /** The player's choice for the loading splash (#43, #97); absent is the default: on for Windows, off for macOS, unavailable elsewhere. */
  loadingSplash?: boolean
  /** Whether the splash can run on this OS at all (Windows and macOS). Derived by GetSettings, never saved. */
  loadingSplashAvailable?: boolean
  /** Whether the next Play shows the splash: the effective value of loadingSplash on this OS. Derived, never saved. */
  loadingSplashOn?: boolean
  /** The slideshow off (issue 142): each chapter shows its own bundled picture, with no timer. Absent is on. */
  staticArt?: boolean
  /** How many wiki pictures each chapter's slideshow has (issue 172): 5, 10 or 20, 0 for all. Absent is 10. */
  wikiPictures?: number
  /** Where the hero's map tool opens a chapter's map (issue 161). Absent is the launcher's own page. */
  mapIn?: MapIn | string
}

/** Where a chapter's map opens: a page in the launcher, or the system browser. */
export type MapIn = 'app' | 'browser'
