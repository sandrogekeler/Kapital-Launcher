import { describe, expect, it } from 'vitest'
import type { PackState } from '../types'
import { packVersion, versionValue } from './packState'

const state = (over: Partial<PackState>): PackState => ({
  chapterId: 'frangfurd',
  installed: true,
  checked: true,
  upToDate: true,
  version: '1.0.0',
  ...over,
})

describe('packState', () => {
  it('names the version, or what it is older than, or nothing', () => {
    expect(versionValue(undefined)).toBe('[PLACEHOLDER]')
    expect(versionValue(state({ installed: false }))).toBe('[PLACEHOLDER]')
    expect(versionValue(state({ checked: false }))).toBe('[PLACEHOLDER]')
    expect(versionValue(state({}))).toBe('1.0.0')
    expect(versionValue(state({ upToDate: false, version: '1.1.0' }))).toBe('older than 1.1.0')
    expect(versionValue(state({ version: '' }))).toBe('[PLACEHOLDER]')
  })

  it('names the version the source serves beside Play, else the manifest version', () => {
    expect(packVersion(state({ version: '1.0.1' }), '1.0.0')).toBe('1.0.1')
    expect(packVersion(state({ checked: false, version: '1.0.1' }), '1.0.0')).toBe('1.0.0')
    expect(packVersion(state({ version: '' }), '1.0.0')).toBe('1.0.0')
    expect(packVersion(undefined, '1.0.0')).toBe('1.0.0')
    expect(packVersion(undefined, null)).toBeNull()
  })
})
