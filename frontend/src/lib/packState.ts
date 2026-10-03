import type { PackState } from '../types'
import { PLACEHOLDER } from './manifest'

/**
 * What the pack state means for the screen (#71): the version the action bar
 * names, and what the Version row says. Every Play syncs the pack, so nothing
 * here asks whether an update is due.
 */

/**
 * The pack version to name beside Play: the one the pack source serves, which
 * the next Play syncs to, else the manifest's when the source is unread.
 */
export const packVersion = (
  state: PackState | undefined,
  manifest: string | null | undefined,
): string | null => (state?.checked && state.version ? state.version : (manifest ?? null))

/**
 * The Version row: the version when the installed pack is the source's,
 * "older than X" when it is behind, the placeholder when nothing is known.
 */
export function versionValue(state: PackState | undefined): string {
  if (!state || !state.installed || !state.checked) return PLACEHOLDER
  const version = state.version || PLACEHOLDER
  return state.upToDate ? version : `older than ${version}`
}
