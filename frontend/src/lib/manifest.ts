import bundled from '../../../data/launcher.json'
import type { Chapter, Manifest } from '../types'

/**
 * The manifest built into this version, the same file main.go embeds. It is
 * what the browser-only preview renders, and what the UI shows until the Go
 * side has answered.
 */
export const BUNDLED_MANIFEST: Manifest = bundled

/** The marker the manifest uses for a fact nobody has settled yet. */
export const PLACEHOLDER = '[PLACEHOLDER]'

export const isPlaceholder = (value: string | null | undefined) =>
  value == null || value === PLACEHOLDER

/**
 * Whether a server address is still unsettled. The manifest writes an unknown
 * address as a host under `.invalid`, the TLD reserved never to resolve, so
 * the status ping reads offline rather than reaching someone else's machine
 * (docs/adr/0005-server-status.md). The UI shows it as a placeholder.
 */
export function isPlaceholderAddress(address: string | null | undefined): boolean {
  if (address == null || address === PLACEHOLDER) return true
  const host = address.trim().replace(/:\d+$/, '').replace(/\.$/, '').toLowerCase()
  return host === 'invalid' || host.endsWith('.invalid')
}

/** A server address formatted for a fact row: the placeholder marker when unsettled. */
export const addressValue = (address: string | null | undefined): string =>
  address == null || isPlaceholderAddress(address) ? PLACEHOLDER : address

/** A count or a version that may be unknown, formatted for a fact row. */
export const factValue = (value: string | number | null | undefined): string =>
  value == null || value === PLACEHOLDER ? PLACEHOLDER : String(value)

export function chapterById(manifest: Manifest, id: string): Chapter | undefined {
  return manifest.chapters.find((c) => c.id === id)
}

/** The verb on the Play button: a chapter that joins its server on launch is joined, the rest are played. */
export const playLabel = (chapter: Chapter) =>
  chapter.server?.joinOnLaunch ? `Join ${chapter.name}` : `Play ${chapter.name}`

/** The one-word pill for the chapter's state, as the reference writes it. */
export const stateLabel = (state: string) =>
  ({ released: 'Released', development: 'In development', planned: 'Planned' })[state] ?? state
