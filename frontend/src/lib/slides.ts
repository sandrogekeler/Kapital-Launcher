import type { Chapter, WikiPage, WikiShot } from '../types'
import { pagesForChapter, pickPage } from './wiki'

/**
 * The chapter's slide (#142): one of the wiki's screenshots of the chapter's
 * era and a wiki post linked to what it shows. Picked again on every switch to
 * the chapter and every SLIDE_INTERVAL_MS while it is open, never the picture
 * or the post just shown when there is a choice. Pure, so the pairing is
 * tested without a store or a random source.
 */
export interface Slide {
  /** The picture's src, served by Go; undefined shows the bundled art. */
  art?: string
  /** The post; undefined shows the manifest's teaser. */
  page?: WikiPage
}

/** How long a slide stays before the next one, while the window is in view. */
export const SLIDE_INTERVAL_MS = 60_000

/** The screenshots of the chapter's era. The wiki's era ids are the chapter names. */
export const shotsForChapter = (shots: readonly WikiShot[], chapter: Chapter): WikiShot[] =>
  shots.filter((s) => s.era === chapter.name)

/**
 * The posts a picture may come with: the page it shows and the pages the wiki
 * relates to that page, those of the chapter's era. A picture with no subject,
 * or whose subject links to nothing of the era, takes any page of the era.
 */
export function postsForShot(
  shot: WikiShot | undefined,
  pages: readonly WikiPage[],
  eraPages: readonly WikiPage[],
): WikiPage[] {
  if (!shot?.subject) return [...eraPages]
  const subject = pages.find((p) => p.id === shot.subject)
  const linked = new Set([shot.subject, ...(subject?.related ?? [])])
  const posts = eraPages.filter((p) => linked.has(p.id))
  return posts.length > 0 ? posts : [...eraPages]
}

/** One of `pool`, chosen by `random` in [0, 1), skipping `skip` when there is a choice. */
function pickOne<T>(pool: readonly T[], skip: (item: T) => boolean, random: () => number) {
  const rest = pool.length > 1 ? pool.filter((x) => !skip(x)) : pool
  const from = rest.length > 0 ? rest : pool
  if (from.length === 0) return undefined
  return from[Math.min(from.length - 1, Math.floor(random() * from.length))]
}

/** The next slide for the chapter, after `previous`. */
export function pickSlide(
  shots: readonly WikiShot[],
  pages: readonly WikiPage[],
  chapter: Chapter,
  previous: Slide | undefined,
  random: () => number = Math.random,
): Slide {
  const eraPages = pagesForChapter([...pages], chapter)
  const shot = pickOne(shotsForChapter(shots, chapter), (s) => s.src === previous?.art, random)
  const page = pickPage(postsForShot(shot, pages, eraPages), previous?.page, random)
  return { art: shot?.src, page }
}

/**
 * Loads a picture before it is shown, so a slide never changes to a picture
 * that is still arriving. A failure is not an error: the slide shows what the
 * webview makes of it.
 */
export async function preload(src: string | undefined): Promise<void> {
  if (!src || typeof Image === 'undefined') return
  const img = new Image()
  img.src = src
  try {
    await img.decode()
  } catch {
    // An image that will not decode is shown as the webview shows it.
  }
}
