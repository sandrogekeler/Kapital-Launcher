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
  version?: string | null
  packwiz?: string | null
  mrpack?: string | null
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

export interface AppSettings {
  theme: Theme | string
  prismExecutable: string
  prismRoot: string
  profileName: string
  lastChapter: string
}
