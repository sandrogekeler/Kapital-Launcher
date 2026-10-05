import { describe, expect, it } from 'vitest'
import { HERO_ART_OPTIONS, heroArtHint, heroArtOf } from './heroArt'

describe('the hero picture choice (issue 195)', () => {
  it('offers the slideshow, the default and the panorama, in that order', () => {
    expect(HERO_ART_OPTIONS.map((o) => [o.value, o.label])).toEqual([
      ['slideshow', 'Slideshow'],
      ['default', 'Default'],
      ['panorama', 'Panorama'],
    ])
  })

  it('reads a stored choice, and anything else as the slideshow, Go default', () => {
    expect(heroArtOf({})).toBe('slideshow')
    expect(heroArtOf({ heroArt: 'slideshow' })).toBe('slideshow')
    expect(heroArtOf({ heroArt: 'default' })).toBe('default')
    expect(heroArtOf({ heroArt: 'panorama' })).toBe('panorama')
    expect(heroArtOf({ heroArt: 'video' })).toBe('slideshow')
    expect(heroArtOf({ heroArt: '' })).toBe('slideshow')
  })

  it('says what each choice is in one short line, with no em dash', () => {
    for (const { value } of HERO_ART_OPTIONS) {
      expect(heroArtHint(value)).not.toMatch(/—/)
      // One line at the row's width, so the rows do not move as the choice changes.
      expect(heroArtHint(value).length).toBeLessThanOrEqual(48)
    }
    expect(heroArtHint('panorama')).toBe("The pack's title screen.")
  })
})
