import { describe, expect, it } from 'vitest'
import { clock, serverLine } from './serverLine'
import type { ServerStatus } from '../types'

const base: ServerStatus = {
  chapterId: 'lichdenstein',
  checked: true,
  online: true,
  players: 4,
  max: 20,
  version: 'Paper 1.20.6',
  motd: '',
  latencyMs: 37,
  checkedAt: '2026-09-28T12:34:00Z',
}

describe('serverLine', () => {
  it('says an unsettled address is pending, whatever the ping said', () => {
    const pending: [string, string] = ['○ No server yet', 'Address pending']
    expect(serverLine(undefined, true)).toEqual(pending)
    expect(serverLine({ ...base, online: false }, true)).toEqual(pending)
    expect(serverLine(base, true)).toEqual(pending)
  })

  it('says checking before the first ping, with a second row kept for the bar', () => {
    expect(serverLine(undefined, false)).toEqual(['○ Checking server', ''])
    expect(serverLine({ ...base, checked: false }, false)).toEqual(['○ Checking server', ''])
  })

  it('says online with players and latency, and no address', () => {
    expect(serverLine(base, false)).toEqual(['● Server online', '4/20 players · 37 ms'])
  })

  it('says offline with the time of the last check, and no address', () => {
    const [state, meta] = serverLine({ ...base, online: false }, false)
    expect(state).toBe('○ Server offline')
    expect(meta).toBe(`as of ${clock(base.checkedAt)}`)
  })

  it('clock falls back on an unreadable timestamp', () => {
    expect(clock('nonsense')).toBe('?')
  })
})
