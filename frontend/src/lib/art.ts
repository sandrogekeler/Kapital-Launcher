import luxemburg from '../assets/screenshots/luxemburg.webp'
import frangfurd from '../assets/screenshots/frangfurd.webp'

/**
 * One screenshot per chapter, from the wiki's public/screenshots. A chapter
 * with none renders the accent grid (`.art-pending` in style.css) instead,
 * which is what Lichdenstein does until it has a capture. Pictures are
 * bundled for now; when the manifest comes from the site (ADR-4) this becomes
 * a lookup by URL and the CSP's img-src grows by that host.
 */
const ART: Record<string, string> = { luxemburg, frangfurd }

export const chapterArt = (id: string): string | undefined => ART[id]
