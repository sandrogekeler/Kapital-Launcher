import { describe, expect, it } from 'vitest'
import type { PackState } from '../types'
import { updateAvailable, versionValue } from './packState'

const state = (over: Partial<PackState>): PackState => ({
  chapterId: 'frangfurd',
  installed: true,
  checked: true,
  upToDate: true,
  version: '1.0.0',
  ...over,
})

describe('packState', () => {
  it('offers an update only for an installed pack known to be behind', () => {
    expect(updateAvailable(undefined)).toBe(false)
    expect(updateAvailable(state({}))).toBe(false)
    expect(updateAvailable(state({ upToDate: false }))).toBe(true)
    expect(updateAvailable(state({ upToDate: false, checked: false }))).toBe(false)
    expect(updateAvailable(state({ upToDate: false, installed: false }))).toBe(false)
  })

  it('names the version, or what it is older than, or nothing', () => {
    expect(versionValue(undefined)).toBe('[PLACEHOLDER]')
    expect(versionValue(state({ installed: false }))).toBe('[PLACEHOLDER]')
    expect(versionValue(state({ checked: false }))).toBe('[PLACEHOLDER]')
    expect(versionValue(state({}))).toBe('1.0.0')
    expect(versionValue(state({ upToDate: false, version: '1.1.0' }))).toBe('older than 1.1.0')
    expect(versionValue(state({ version: '' }))).toBe('[PLACEHOLDER]')
  })
})
