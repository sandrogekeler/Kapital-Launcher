import luxemburg from '../assets/screenshots/luxemburg.webp'
import frangfurd from '../assets/screenshots/frangfurd.webp'
import lichdenstein from '../assets/screenshots/lichdenstein.webp'
import luxemburgTitle from '../assets/titles/luxemburg.png'
import frangfurdTitle from '../assets/titles/frangfurd.png'
import lichdensteinTitle from '../assets/titles/lichdenstein.png'
import luxemburgIcon from '../assets/icons/luxemburg.png'
import frangfurdIcon from '../assets/icons/frangfurd.png'
import lichdensteinIcon from '../assets/icons/lichdenstein.png'

/**
 * One screenshot per chapter, the first of each in the wiki's
 * public/screenshots/<chapter>/. A chapter with none renders the accent grid
 * (`.art-pending` in style.css) instead. Pictures are
 * bundled for now; when the manifest comes from the site (ADR-4) this becomes
 * a lookup by URL and the CSP's img-src grows by that host.
 */
const ART: Record<string, string> = { luxemburg, lichdenstein, frangfurd }

export const chapterArt = (id: string): string | undefined => ART[id]

/**
 * The chapter's name as its title artwork (#69), from the author's art
 * folders, scaled to 1200 px wide: twice the hero's text width, sharp on a
 * HiDPI screen. PNG keeps the pixel edges and the transparency. A chapter
 * without one keeps its name in the display face.
 */
const TITLE_ART: Record<string, string> = {
  luxemburg: luxemburgTitle,
  lichdenstein: lichdensteinTitle,
  frangfurd: frangfurdTitle,
}

export const chapterTitleArt = (id: string): string | undefined => TITLE_ART[id]

/**
 * The pack's own icon for its nav row (#77), from the author's art folders
 * at 160 px: four times the 40 px thumbnail. A chapter without one shows
 * its screenshot there instead.
 */
const ICON: Record<string, string> = {
  luxemburg: luxemburgIcon,
  lichdenstein: lichdensteinIcon,
  frangfurd: frangfurdIcon,
}

export const chapterIcon = (id: string): string | undefined => ICON[id]
