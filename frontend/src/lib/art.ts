import luxemburg from '../assets/screenshots/luxemburg.webp'
import frangfurd from '../assets/screenshots/frangfurd.webp'
import lichdenstein from '../assets/screenshots/lichdenstein.webp'

/**
 * One screenshot per chapter, the first of each in the wiki's
 * public/screenshots/<chapter>/. A chapter with none renders the accent grid
 * (`.art-pending` in style.css) instead. Pictures are
 * bundled for now; when the manifest comes from the site (ADR-4) this becomes
 * a lookup by URL and the CSP's img-src grows by that host.
 */
const ART: Record<string, string> = { luxemburg, lichdenstein, frangfurd }

export const chapterArt = (id: string): string | undefined => ART[id]
