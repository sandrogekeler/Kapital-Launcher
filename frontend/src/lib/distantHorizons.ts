import type { Chapter } from '../types'

/** The jar prefix the manifest's quick toggle gives Distant Horizons. */
const DH_PREFIX = 'DistantHorizons-'

/**
 * Whether the chapter's pack has Distant Horizons, by the manifest's own toggle for it: where the
 * world data card belongs (issue 136).
 */
export function hasDistantHorizons(chapter: Chapter): boolean {
  return (chapter.pack.toggles ?? []).some((t) => t.jarPrefix === DH_PREFIX)
}
