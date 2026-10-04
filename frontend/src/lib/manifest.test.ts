import { describe, expect, it } from 'vitest'
import {
  BUNDLED_MANIFEST,
  addressValue,
  chapterById,
  chosenAddress,
  chosenLabel,
  factValue,
  isPlaceholder,
  isPlaceholderAddress,
  playLabel,
  sizeValue,
  knownFacts,
  unsetLabel,
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

  it('treats an address under .invalid as a placeholder, and a real one as itself', () => {
    for (const unsettled of [
      'placeholder.invalid',
      'PLACEHOLDER.INVALID',
      'lichdenstein.invalid:25565',
      'placeholder.invalid.',
      'invalid',
      '[PLACEHOLDER]',
      null,
      undefined,
    ]) {
      expect(isPlaceholderAddress(unsettled), String(unsettled)).toBe(true)
      expect(addressValue(unsettled)).toBe('[PLACEHOLDER]')
    }
    for (const real of [
      'rails-enjoyed.tun.ply.gg',
      'play.example:25565',
      'invalid.example',
      'notinvalid',
    ]) {
      expect(isPlaceholderAddress(real), real).toBe(false)
      expect(addressValue(real)).toBe(real)
    }
  })

  it('resolves the address in use like Go does: the saved label, else the first', () => {
    const server = {
      addresses: [
        { label: 'Global', address: 'global.example' },
        { label: 'Germany', address: 'de.example:25570' },
      ],
      joinOnLaunch: false,
      software: 'Paper',
    }
    expect(chosenAddress(server, undefined)).toBe('global.example')
    expect(chosenAddress(server, 'Germany')).toBe('de.example:25570')
    expect(chosenAddress(server, 'Asia')).toBe('global.example')
    expect(chosenAddress(server, 'de.example:25570')).toBe('global.example')
    expect(chosenLabel(server, 'Germany')).toBe('Germany')
    expect(chosenLabel(server, 'Asia')).toBe('Global')
    expect(chosenAddress(null, 'Germany')).toBeUndefined()
    expect(chosenLabel(undefined, undefined)).toBeUndefined()
  })

  it('says in words what an unsettled fact is: not installed, or unknown', () => {
    expect(unsetLabel(false)).toBe('Not installed')
    expect(unsetLabel(true)).toBe('Unknown')
    expect(unsetLabel(undefined)).toBe('Unknown')
  })

  it('joins the facts that are settled and says nothing when none is', () => {
    expect(knownFacts('NeoForge', '1.21.1')).toBe('NeoForge 1.21.1')
    expect(knownFacts('[PLACEHOLDER]', '1.21.1')).toBe('1.21.1')
    expect(knownFacts(96)).toBe('96')
    expect(knownFacts('[PLACEHOLDER]', null, undefined)).toBeNull()
  })
})

describe('sizeValue', () => {
  it('formats a size on disk for the fact row', () => {
    expect(sizeValue(undefined)).toBe('[PLACEHOLDER]')
    expect(sizeValue(-1)).toBe('[PLACEHOLDER]')
    expect(sizeValue(0)).toBe('0 MB')
    expect(sizeValue(512_400_000)).toBe('512 MB')
    expect(sizeValue(1e9)).toBe('1.0 GB')
    expect(sizeValue(4_250_000_000)).toBe('4.3 GB')
  })
})
