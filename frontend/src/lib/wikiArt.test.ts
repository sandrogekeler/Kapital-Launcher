import { describe, expect, it } from 'vitest'
import { BUNDLED_MANIFEST } from './manifest'
import {
  FALLBACK_PICTURE_BYTES,
  choiceOf,
  estimateBytes,
  numberOf,
  picturesHint,
  picturesOf,
} from './wikiArt'

const chapters = BUNDLED_MANIFEST.chapters
const stats = { avgBytes: 102_400, pools: { Luxemburg: 60, Frangfurd: 73, Lichdenstein: 4 } }

describe('the number of pictures', () => {
  it('is 10 when unset, 0 meaning all', () => {
    expect(picturesOf({})).toBe(10)
    expect(picturesOf({ wikiPictures: 0 })).toBe(0)
    expect(picturesOf({ wikiPictures: 20 })).toBe(20)
  })

  it('maps the stored number to the control and back', () => {
    expect([0, 5, 10, 20].map(choiceOf)).toEqual(['all', '5', '10', '20'])
    expect(choiceOf(7)).toBe('10')
    expect((['5', '10', '20', 'all'] as const).map(numberOf)).toEqual([5, 10, 20, 0])
  })
})

describe('the space estimate', () => {
  it('is the number per chapter, or the pool where it is smaller, times the average picture', () => {
    // 10 + 10 + 4 pictures.
    expect(estimateBytes(10, chapters, stats)).toBe(24 * 102_400)
    expect(estimateBytes(0, chapters, stats)).toBe(137 * 102_400)
  })

  it('goes by the number and 100 KB before the cache or the export has anything to say', () => {
    expect(estimateBytes(5, chapters, null)).toBe(5 * chapters.length * FALLBACK_PICTURE_BYTES)
    expect(estimateBytes(5, chapters, { avgBytes: 0, pools: {} })).toBe(
      5 * chapters.length * FALLBACK_PICTURE_BYTES,
    )
    expect(estimateBytes(0, chapters, null)).toBeNull()
  })

  it('reads as a line under the control', () => {
    expect(picturesHint(10, chapters, stats)).toBe(
      'About 2.3 MB on disk. A new set is picked each day.',
    )
    expect(picturesHint(10, chapters, null)).toBe(
      'About 2.9 MB on disk. A new set is picked each day.',
    )
    expect(picturesHint(0, chapters, stats)).toBe(
      'About 13 MB on disk, every picture the wiki has.',
    )
    expect(picturesHint(0, chapters, null)).toMatch(/depends/)
  })
})
