import bundled from '../../../data/launcher.json'
import type { Chapter, Manifest, Server } from '../types'

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

/**
 * The address a chapter's server is reached at: the one the saved choice (a
 * label) names, else the first, as Go's services.ServerAddress resolves it
 * (issue 151). Used only to tell a placeholder from a real address: the
 * address itself is never shown.
 */
export function chosenAddress(
  server: Server | null | undefined,
  choice: string | undefined,
): string | undefined {
  const list = server?.addresses ?? []
  return (list.find((a) => a.label === choice) ?? list[0])?.address
}

/** The label of the address in use: the saved choice when the manifest lists it, else the first. */
export function chosenLabel(
  server: Server | null | undefined,
  choice: string | undefined,
): string | undefined {
  const list = server?.addresses ?? []
  return (list.find((a) => a.label === choice) ?? list[0])?.label
}

/** A server address formatted for a fact row: the placeholder marker when unsettled. */
export const addressValue = (address: string | null | undefined): string =>
  address == null || isPlaceholderAddress(address) ? PLACEHOLDER : address

/** A count or a version that may be unknown, formatted for a fact row. */
export const factValue = (value: string | number | null | undefined): string =>
  value == null || value === PLACEHOLDER ? PLACEHOLDER : String(value)

/**
 * What a fact row says in place of a placeholder: a chapter that is not
 * installed has no version and no size yet, so it says so, and any other
 * fact nobody has settled is unknown.
 */
export const unsetLabel = (installed: boolean | undefined): string =>
  installed === false ? 'Not installed' : 'Unknown'

/** The facts that are settled, joined by a space for a chip; null when none is. */
export function knownFacts(...values: (string | number | null | undefined)[]): string | null {
  const known = values.filter((v) => !isPlaceholder(v == null ? null : String(v)))
  return known.length > 0 ? known.join(' ') : null
}

/**
 * A size on disk for a fact row: gigabytes with one decimal from a gigabyte
 * up, whole megabytes below, the placeholder when nothing is installed to
 * measure (#57). Decimal units, as the OS file dialogs show them.
 */
export function sizeValue(bytes: number | undefined): string {
  if (bytes == null || bytes < 0) return PLACEHOLDER
  const gb = bytes / 1e9
  if (gb >= 1) return `${gb.toFixed(1)} GB`
  return `${Math.round(bytes / 1e6)} MB`
}

export function chapterById(manifest: Manifest, id: string): Chapter | undefined {
  return manifest.chapters.find((c) => c.id === id)
}

/** The verb on the Play button: a chapter that joins its server on launch is joined, the rest are played. */
export const playLabel = (chapter: Chapter) =>
  chapter.server?.joinOnLaunch ? `Join ${chapter.name}` : `Play ${chapter.name}`

/** Whether the chapter's pack is hosted, so Install has something to install (#25). */
export const isPublished = (chapter: Chapter) => chapter.pack.packwiz != null

/** The verb on the button when the chapter's instance is missing. */
export const installLabel = (chapter: Chapter) => `Install ${chapter.name}`
