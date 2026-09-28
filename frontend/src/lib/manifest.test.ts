import { describe, expect, it } from 'vitest'
import {
  BUNDLED_MANIFEST,
  chapterById,
  factValue,
  isPlaceholder,
  playLabel,
  stateLabel,
} from './manifest'
import { CHAPTER_IDS } from '../styles/tokens'

describe('bundled manifest', () => {
  it('names only chapters the token set has an accent for', () => {
    for (const c of BUNDLED_MANIFEST.chapters) {
      expect(CHAPTER_IDS).toContain(c.id)
    }
  })

  it('finds a chapter by id', () => {
    expect(chapterById(BUNDLED_MANIFEST, 'frangfurd')?.name).toBe('Frangfurd')
    expect(chapterById(BUNDLED_MANIFEST, 'atlantis')).toBeUndefined()
  })
})

describe('labels', () => {
  it('joins a server and plays a pack', () => {
    const [lux, lic, fra] = BUNDLED_MANIFEST.chapters
    expect(playLabel(lux!)).toBe('Play Luxemburg')
    expect(playLabel(lic!)).toBe('Join Lichdenstein')
    expect(playLabel(fra!)).toBe('Play Frangfurd')
  })

  it('renders unknown facts as the placeholder marker', () => {
    expect(isPlaceholder(null)).toBe(true)
    expect(isPlaceholder('[PLACEHOLDER]')).toBe(true)
    expect(isPlaceholder('NeoForge')).toBe(false)
    expect(factValue(null)).toBe('[PLACEHOLDER]')
    expect(factValue(42)).toBe('42')
  })

  it('spells states as the reference does', () => {
    expect(stateLabel('development')).toBe('In development')
    expect(stateLabel('odd')).toBe('odd')
  })
})
