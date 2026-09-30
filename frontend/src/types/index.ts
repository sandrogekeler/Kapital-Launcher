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

export interface AppSettings {
  theme: Theme | string
  prismExecutable: string
  prismRoot: string
  profileName: string
  lastChapter: string
  /** Chapter id to a local packwiz serve address (#41), edited on the settings screen (#5). */
  packOverrides?: Record<string, string>
}
