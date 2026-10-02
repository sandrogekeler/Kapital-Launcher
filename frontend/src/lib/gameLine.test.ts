import { describe, expect, it } from 'vitest'
import { gameLine } from './gameLine'
import type { GameFailReason, GamePhase, GameState } from '../types'

const at = (phase: GamePhase, exitCode?: number): GameState => ({
  chapterId: 'frangfurd',
  phase,
  since: '2026-10-01T10:00:00Z',
  startedAt: '2026-10-01T09:59:00Z',
  ...(exitCode === undefined ? {} : { exitCode }),
})

describe('gameLine', () => {
  it('names each phase of a start in order', () => {
    expect(gameLine(at('starting'), 'Frangfurd')).toEqual([
      '◐ Starting',
      'Prism is getting Frangfurd ready',
      'text-accent',
    ])
    expect(gameLine(at('mods'), 'Frangfurd')).toEqual([
      '◐ Loading mods',
      'The game has started',
      'text-accent',
    ])
    expect(gameLine(at('window'), 'Frangfurd')).toEqual([
      '◐ Loading mods',
      'The game window is up',
      'text-accent',
    ])
    expect(gameLine(at('resources'), 'Frangfurd')).toEqual([
      '◐ Loading resources',
      'Almost there',
      'text-accent',
    ])
  })

  it('says playing and closing', () => {
    expect(gameLine(at('running'), 'Frangfurd')).toEqual([
      '● Playing',
      'Frangfurd is running',
      'text-accent',
    ])
    expect(gameLine(at('stopping'), 'Frangfurd')).toEqual([
      '◐ Closing',
      'Saving and shutting down',
      'text-accent',
    ])
  })

  it('says a crash with its exit code, or without one when there is none', () => {
    expect(gameLine(at('crashed', 1), 'Frangfurd')).toEqual([
      '○ The game stopped',
      'It closed before it finished, exit code 1',
      'text-danger',
    ])
    expect(gameLine(at('crashed', 0), 'Frangfurd')?.[1]).toContain('exit code 0')
    expect(gameLine(at('crashed'), 'Frangfurd')).toEqual([
      '○ The game stopped',
      'It closed before it finished',
      'text-danger',
    ])
  })

  it('says a game that never started', () => {
    expect(gameLine(at('failed'), 'Frangfurd')).toEqual([
      '○ The game did not start',
      'Prism gave up or never started it',
      'text-danger',
    ])
  })

  it('says why a start failed when Prism said so', () => {
    const failed = (reason: GameFailReason): GameState => ({ ...at('failed'), reason })
    expect(gameLine(failed('packsync'), 'Frangfurd')).toEqual([
      '○ The game did not start',
      "The pack could not be synced. Prism's window has the details",
      'text-danger',
    ])
    expect(gameLine(failed('launch'), 'Frangfurd')).toEqual([
      '○ The game did not start',
      'Prism stopped before the game. Its window has the details',
      'text-danger',
    ])
  })

  it('says a run the player stopped without the error tone', () => {
    expect(gameLine({ ...at('failed'), reason: 'stopped' }, 'Frangfurd')).toEqual([
      '○ The game did not start',
      'Stopped from the launcher',
      'text-fg-muted',
    ])
    expect(gameLine({ ...at('crashed', 1), reason: 'stopped' }, 'Frangfurd')).toEqual([
      '○ The game was stopped',
      'Stopped from the launcher',
      'text-fg-muted',
    ])
  })

  it('has nothing to say for idle, closed or an unknown state', () => {
    expect(gameLine(at('idle'), 'Frangfurd')).toBeNull()
    expect(gameLine(at('closed'), 'Frangfurd')).toBeNull()
    expect(gameLine(undefined, 'Frangfurd')).toBeNull()
  })

  it('uses no em dash in any line', () => {
    const phases: GamePhase[] = [
      'starting',
      'mods',
      'window',
      'resources',
      'running',
      'stopping',
      'crashed',
      'failed',
    ]
    for (const p of phases) {
      expect(gameLine(at(p, 2), 'Frangfurd')?.join(' ')).not.toContain('—')
    }
    for (const reason of ['packsync', 'launch', 'stopped'] as const) {
      expect(gameLine({ ...at('failed'), reason }, 'Frangfurd')?.join(' ')).not.toContain('—')
    }
    expect(gameLine({ ...at('crashed'), reason: 'stopped' }, 'Frangfurd')?.join(' ')).not.toContain(
      '—',
    )
  })
})
