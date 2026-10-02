import { describe, expect, it } from 'vitest'
import { seconds, timeline } from './runReport'
import type { GameState, RunReport } from '../types'

const game = (over: Partial<GameState> = {}): GameState => ({
  chapterId: 'frangfurd',
  phase: 'crashed',
  startedAt: '2026-10-02T10:00:00Z',
  since: '2026-10-02T10:00:31Z',
  ...over,
})

const report = (over: Partial<RunReport> = {}): RunReport => ({
  game: game(),
  phases: [{ phase: 'starting', ms: 0 }],
  logTail: '',
  logLines: 0,
  logTruncated: false,
  crashReport: '',
  consoleAvailable: false,
  ...over,
})

describe('seconds', () => {
  it('keeps a decimal under ten and rounds after', () => {
    expect(seconds(4200)).toBe('4.2 s')
    expect(seconds(9800)).toBe('9.8 s')
    expect(seconds(31_400)).toBe('31 s')
    expect(seconds(-5)).toBe('0.0 s')
  })
})

describe('timeline', () => {
  it('lists each phase reached and how the run ended, in one line', () => {
    const r = report({
      phases: [
        { phase: 'starting', ms: 0 },
        { phase: 'mods', ms: 4200 },
        { phase: 'window', ms: 9800 },
      ],
    })
    expect(timeline(r)).toBe('mods 4.2 s, window 9.8 s, crashed 31 s')
  })

  it('has only the end for a start that failed before the game', () => {
    expect(
      timeline(report({ game: game({ phase: 'failed', since: '2026-10-02T10:00:12Z' }) })),
    ).toBe('failed 12 s')
  })

  it('does not name a run that is still going as ended', () => {
    const r = report({
      game: game({ phase: 'window' }),
      phases: [
        { phase: 'starting', ms: 0 },
        { phase: 'mods', ms: 4200 },
      ],
    })
    expect(timeline(r)).toBe('mods 4.2 s')
  })

  it('is null when nothing is known', () => {
    expect(timeline(report({ game: game({ phase: 'starting' }) }))).toBeNull()
    expect(timeline(report({ game: game({ startedAt: '' }) }))).toBeNull()
  })
})
