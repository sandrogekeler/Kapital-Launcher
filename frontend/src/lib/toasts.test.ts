import { describe, expect, it } from 'vitest'
import { errorToasts } from './toasts'
import type { GamePhase, GameState } from '../types'

const at = (phase: GamePhase, over: Partial<GameState> = {}): GameState => ({
  chapterId: 'frangfurd',
  phase,
  since: '2026-10-01T10:00:00Z',
  startedAt: '2026-10-01T09:59:00Z',
  ...over,
})

const name = (id: string) => (id === 'frangfurd' ? 'Frangfurd' : id)
const none = { engineError: null, stopErrors: {}, games: {}, chapterName: name }

describe('errorToasts', () => {
  it('has nothing to say when nothing went wrong', () => {
    expect(errorToasts(none)).toEqual([])
  })

  it("carries the backend's refusal of a launch or an install in its own words", () => {
    expect(errorToasts({ ...none, engineError: 'launch frangfurd: access is denied' })).toEqual([
      {
        id: 'engine',
        stamp: 'launch frangfurd: access is denied',
        title: '○ Something went wrong',
        detail: 'launch frangfurd: access is denied',
        tone: 'danger',
      },
    ])
  })

  it('files a refused stop under its chapter', () => {
    const toasts = errorToasts({ ...none, stopErrors: { frangfurd: 'no game to stop' } })
    expect(toasts).toEqual([
      expect.objectContaining({
        id: 'stop:frangfurd',
        title: '○ The game could not be stopped',
        detail: 'no game to stop',
        tone: 'danger',
      }),
    ])
  })

  it('offers Details on a run that crashed or never started, stamped by its end', () => {
    const toasts = errorToasts({
      ...none,
      games: { frangfurd: at('failed', { reason: 'packsync' }) },
    })
    expect(toasts).toEqual([
      {
        id: 'game:frangfurd',
        stamp: 'failed:2026-10-01T10:00:00Z',
        title: '○ The game did not start',
        detail: 'The pack could not be synced',
        tone: 'danger',
        reportFor: 'frangfurd',
      },
    ])
    expect(errorToasts({ ...none, games: { frangfurd: at('crashed', { exitCode: 1 }) } })).toEqual([
      expect.objectContaining({
        title: '○ The game stopped',
        detail: 'It closed before it finished, exit code 1',
        reportFor: 'frangfurd',
      }),
    ])
  })

  it('confirms a stop the player asked for in the muted tone, with no Details', () => {
    const [toast] = errorToasts({
      ...none,
      games: { frangfurd: at('crashed', { reason: 'stopped' }) },
    })
    expect(toast).toEqual({
      id: 'game:frangfurd',
      stamp: 'crashed:2026-10-01T10:00:00Z',
      title: '○ The game was stopped',
      detail: 'Stopped from the launcher',
      tone: 'muted',
    })
  })

  it.each<GamePhase>([
    'idle',
    'starting',
    'mods',
    'window',
    'resources',
    'running',
    'stopping',
    'closed',
  ])('says nothing about a game that is %s', (phase) => {
    expect(errorToasts({ ...none, games: { frangfurd: at(phase) } })).toEqual([])
  })

  it('lists every source, engine first', () => {
    const toasts = errorToasts({
      engineError: 'refused',
      stopErrors: { luxemburg: 'no game' },
      games: { frangfurd: at('failed') },
      chapterName: name,
    })
    expect(toasts.map((t) => t.id)).toEqual(['engine', 'stop:luxemburg', 'game:frangfurd'])
  })
})
