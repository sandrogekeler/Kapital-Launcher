import type { AppSettings, HeroArt } from '../types'

/**
 * What the hero shows behind a chapter (issue 195): the wiki's pictures cycling,
 * the chapter's own picture, or its title-screen panorama as a turning cube.
 * Go stores the choice; this is the wording and the reading of it, pure and
 * tested here.
 */

/** The choices Settings offers, in the order the control shows them. */
export const HERO_ART_OPTIONS: readonly { value: HeroArt; label: string }[] = [
  { value: 'slideshow', label: 'Slideshow' },
  { value: 'default', label: 'Default' },
  { value: 'panorama', label: 'Panorama' },
]

/** The line under the control: what the chosen picture is, and where the panorama falls short. */
export function heroArtHint(choice: HeroArt): string {
  switch (choice) {
    case 'default':
      return "Each chapter's own picture."
    case 'panorama':
      return "The pack's title screen, turning slowly. A chapter without one shows its own picture."
    default:
      return 'Pictures from the wiki, changing every minute.'
  }
}

/** The choice a setting stands for: anything but the other two is the slideshow, Go's default. */
export function heroArtOf(s: Pick<AppSettings, 'heroArt'>): HeroArt {
  return s.heroArt === 'default' || s.heroArt === 'panorama' ? s.heroArt : 'slideshow'
}
