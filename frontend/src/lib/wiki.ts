import type { Chapter, WikiPage } from '../types'

/**
 * Which wiki page the "From the wiki" panel shows (#58): a random page of the
 * chapter's era on every switch, never the one just shown when there is a
 * choice. Pure, so the picking is tested without a store or a random source.
 */

/** The pages of the chapter's era. The wiki's era ids are the chapter names. */
export const pagesForChapter = (pages: readonly WikiPage[], chapter: Chapter): WikiPage[] =>
  pages.filter((p) => p.eras.includes(chapter.name))

/**
 * One of `candidates`, chosen by `random` in [0, 1), skipping `previous` when
 * there is more than one to choose from. Undefined when there is nothing to
 * pick, which is when the panel falls back to the manifest's teaser.
 */
export function pickPage(
  candidates: readonly WikiPage[],
  previous: WikiPage | undefined,
  random: () => number = Math.random,
): WikiPage | undefined {
  const pool =
    candidates.length > 1 && previous
      ? candidates.filter((p) => p.url !== previous.url)
      : candidates
  if (pool.length === 0) return undefined
  return pool[Math.min(pool.length - 1, Math.floor(random() * pool.length))]
}
