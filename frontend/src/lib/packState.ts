import type { PackState } from '../types'
import { PLACEHOLDER } from './manifest'

/**
 * What the pack state means for the screen (#71): whether Play should read
 * "Update and play", and what the Version row says.
 */

/** The installed pack is behind its source. */
export const updateAvailable = (state: PackState | undefined): boolean =>
  state !== undefined && state.installed && state.checked && !state.upToDate

/**
 * The Version row: the version when the installed pack is the source's,
 * "older than X" when it is behind, the placeholder when nothing is known.
 */
export function versionValue(state: PackState | undefined): string {
  if (!state || !state.installed || !state.checked) return PLACEHOLDER
  const version = state.version || PLACEHOLDER
  return state.upToDate ? version : `older than ${version}`
}
