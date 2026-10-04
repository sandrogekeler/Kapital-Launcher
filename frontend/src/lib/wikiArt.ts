import type { AppSettings, Chapter, WikiArtStats } from '../types'
import { formatBytes } from './bytes'

/**
 * How many of the wiki's pictures each chapter's slideshow has, and what that
 * costs on disk (issue 172). Go draws and downloads the set; this is the wording
 * and the arithmetic of the settings screen, pure and tested here.
 */

/** The choices Settings offers; 'all' is 0 in the settings file and in Go. */
export type PicturesChoice = '5' | '10' | '20' | 'all'

export const WIKI_PICTURES_OPTIONS: readonly { value: PicturesChoice; label: string }[] = [
  { value: '5', label: '5' },
  { value: '10', label: '10' },
  { value: '20', label: '20' },
  { value: 'all', label: 'All' },
]

/** Go's default when the setting is absent (services.DefaultWikiPictures). */
export const DEFAULT_WIKI_PICTURES = 10

/** What a picture is taken to weigh before the cache can say (Go's defaultArtBytes). */
export const FALLBACK_PICTURE_BYTES = 100 * 1024

/** The number a setting stands for: 0 is all. */
export const picturesOf = (s: Pick<AppSettings, 'wikiPictures'>): number =>
  s.wikiPictures ?? DEFAULT_WIKI_PICTURES

/** The choice the control shows for the stored number. */
export function choiceOf(n: number): PicturesChoice {
  if (n === 0) return 'all'
  return n === 5 || n === 20 ? (String(n) as PicturesChoice) : '10'
}

/** The number to store for a choice. */
export const numberOf = (choice: PicturesChoice): number => (choice === 'all' ? 0 : Number(choice))

/**
 * The space a number of pictures per chapter takes: each chapter's share (the
 * number, or what its pool has if that is less) times the cache's average
 * picture. With no pools known the number alone is the share, and for all of
 * them nothing can be said (null).
 */
export function estimateBytes(
  count: number,
  chapters: readonly Chapter[],
  stats: WikiArtStats | null,
): number | null {
  const avg = stats?.avgBytes && stats.avgBytes > 0 ? stats.avgBytes : FALLBACK_PICTURE_BYTES
  const pools = stats?.pools ?? {}
  if (Object.keys(pools).length === 0) {
    return count === 0 ? null : count * chapters.length * avg
  }
  const pictures = chapters.reduce((sum, c) => {
    const pool = pools[c.name] ?? 0
    return sum + (count === 0 ? pool : Math.min(count, pool))
  }, 0)
  return pictures * avg
}

/** The line under the control. */
export function picturesHint(
  count: number,
  chapters: readonly Chapter[],
  stats: WikiArtStats | null,
): string {
  const bytes = estimateBytes(count, chapters, stats)
  if (count === 0) {
    return bytes === null
      ? 'Every picture the wiki has, so the space depends on how many it has.'
      : `About ${formatBytes(bytes)} on disk, every picture the wiki has.`
  }
  return `About ${formatBytes(bytes ?? 0)} on disk. A new set is picked each day.`
}
