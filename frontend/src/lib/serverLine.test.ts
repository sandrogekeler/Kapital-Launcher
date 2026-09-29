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
    expect(serverLine(undefined, 'placeholder.invalid')).toEqual(pending)
    expect(serverLine({ ...base, online: false }, 'placeholder.invalid')).toEqual(pending)
    expect(serverLine(base, 'lichdenstein.invalid:25565')).toEqual(pending)
  })

  it('says checking before the first ping', () => {
    expect(serverLine(undefined, 'play.example')).toEqual(['○ Checking server', 'play.example'])
    expect(serverLine({ ...base, checked: false }, 'play.example')[0]).toBe('○ Checking server')
  })

  it('says online with players and latency', () => {
    expect(serverLine(base, 'play.example:25565')).toEqual([
      '● Server online',
      'play.example:25565 · 4/20 players · 37 ms',
    ])
  })

  it('says offline with the time of the last check', () => {
    const [state, meta] = serverLine({ ...base, online: false }, 'play.example')
    expect(state).toBe('○ Server offline')
    expect(meta.startsWith('play.example · as of ')).toBe(true)
    expect(meta.endsWith(clock(base.checkedAt))).toBe(true)
  })

  it('clock falls back on an unreadable timestamp', () => {
    expect(clock('nonsense')).toBe('?')
  })
})
