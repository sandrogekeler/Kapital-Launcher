import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as App from '../../wailsjs/go/main/App'
import * as Runtime from '../../wailsjs/runtime/runtime'
import { EVENT_GAME_STATE, isActive, selectGame, useGameStore } from './useGameStore'
import type { GamePhase, GameState } from '../types'

vi.mock('../../wailsjs/go/main/App')
vi.mock('../../wailsjs/runtime/runtime')

const attachBridge = () => Object.assign(window, { go: {} })
const detachBridge = () => {
  delete (window as unknown as { go?: unknown }).go
}

const running: GameState = {
  chapterId: 'frangfurd',
  phase: 'running',
  since: '2026-10-01T10:00:00Z',
  startedAt: '2026-10-01T09:59:00Z',
}

describe('useGameStore', () => {
  beforeEach(() => {
    useGameStore.setState({ states: {}, stopErrors: {} })
    vi.mocked(App.StopGame).mockReset()
    vi.mocked(App.GetGameStates).mockReset()
    vi.mocked(Runtime.EventsOn).mockReset()
    vi.mocked(Runtime.EventsOff).mockReset()
  })
  afterEach(detachBridge)

  it('files a state under its chapter and drops one without', () => {
    useGameStore.getState().receive(running)
    useGameStore.getState().receive({ ...running, chapterId: '' })
    expect(selectGame('frangfurd')(useGameStore.getState())).toEqual(running)
    expect(selectGame('lichdenstein')(useGameStore.getState())).toBeUndefined()
    expect(Object.keys(useGameStore.getState().states)).toEqual(['frangfurd'])
  })

  it('loads every chapter state from Go', async () => {
    attachBridge()
    vi.mocked(App.GetGameStates).mockResolvedValue([
      running,
      { ...running, chapterId: 'lichdenstein', phase: 'idle', startedAt: '' },
    ] as never)
    await useGameStore.getState().load()
    expect(useGameStore.getState().states.frangfurd?.phase).toBe('running')
    expect(useGameStore.getState().states.lichdenstein?.phase).toBe('idle')
  })

  it('degrades to nothing known without a bridge or when the read fails', async () => {
    vi.mocked(App.GetGameStates).mockImplementation(() => {
      throw new TypeError('no bridge')
    })
    await useGameStore.getState().load()
    expect(useGameStore.getState().states).toEqual({})
    expect(useGameStore.getState().listen()).toBeTypeOf('function')
    expect(Runtime.EventsOn).not.toHaveBeenCalled()

    attachBridge()
    vi.mocked(App.GetGameStates).mockRejectedValue(new Error('boom'))
    await useGameStore.getState().load()
    expect(useGameStore.getState().states).toEqual({})
  })

  it('listens to the runtime event with a bridge and unsubscribes', () => {
    attachBridge()
    let handler: ((s: GameState) => void) | undefined
    vi.mocked(Runtime.EventsOn).mockImplementation((_name, cb) => {
      handler = cb as (s: GameState) => void
      return () => undefined
    })
    const off = useGameStore.getState().listen()
    expect(Runtime.EventsOn).toHaveBeenCalledWith(EVENT_GAME_STATE, expect.any(Function))
    handler?.({ ...running, phase: 'crashed', exitCode: 1 })
    expect(useGameStore.getState().states.frangfurd?.exitCode).toBe(1)
    off()
    expect(Runtime.EventsOff).toHaveBeenCalledWith(EVENT_GAME_STATE)
  })

  it('counts the first six phases as active and the rest as not', () => {
    const active: GamePhase[] = ['starting', 'mods', 'window', 'resources', 'running', 'stopping']
    const over: GamePhase[] = ['idle', 'closed', 'crashed', 'failed']
    expect(active.every(isActive)).toBe(true)
    expect(over.some(isActive)).toBe(false)
    expect(isActive(undefined)).toBe(false)
  })

  it('stops a chapter through the binding, and says nothing when it works', async () => {
    attachBridge()
    vi.mocked(App.StopGame).mockResolvedValue(running as never)
    await useGameStore.getState().stop('frangfurd')
    expect(App.StopGame).toHaveBeenCalledWith('frangfurd')
    expect(useGameStore.getState().stopErrors).toEqual({})
  })

  it('records a refused stop under its chapter and rethrows it, until the game moves', async () => {
    attachBridge()
    vi.mocked(App.StopGame).mockRejectedValue(new Error('Frangfurd has no game to stop'))
    await expect(useGameStore.getState().stop('frangfurd')).rejects.toThrow('no game to stop')
    expect(useGameStore.getState().stopErrors).toEqual({
      frangfurd: 'Frangfurd has no game to stop',
    })
    useGameStore.getState().receive({ ...running, chapterId: 'lichdenstein' })
    expect(useGameStore.getState().stopErrors.frangfurd).toBeDefined()
    useGameStore.getState().receive({ ...running, phase: 'crashed', reason: 'stopped' })
    expect(useGameStore.getState().stopErrors).toEqual({})
  })

  it('reads a run report through the binding once per call, and keeps none', async () => {
    attachBridge()
    const report = {
      game: { ...running, phase: 'crashed' },
      phases: [{ phase: 'starting', ms: 0 }],
      logTail: 'line\n',
      logLines: 1,
      logTruncated: false,
      crashReport: '',
      consoleAvailable: false,
    }
    vi.mocked(App.GetRunReport).mockResolvedValue(report as never)
    const before = useGameStore.getState().states
    await expect(useGameStore.getState().report('frangfurd')).resolves.toEqual(report)
    expect(App.GetRunReport).toHaveBeenCalledExactlyOnceWith('frangfurd')
    // Nothing of the log is held by the store.
    expect(useGameStore.getState().states).toBe(before)
    expect(JSON.stringify(useGameStore.getState())).not.toContain('line')
  })

  it('degrades a run report to null without a bridge, and rethrows a real refusal', async () => {
    vi.mocked(App.GetRunReport).mockImplementation(() => {
      throw new TypeError('no bridge')
    })
    await expect(useGameStore.getState().report('frangfurd')).resolves.toBeNull()
    attachBridge()
    vi.mocked(App.GetRunReport).mockRejectedValue(new Error('there is no run to report on'))
    await expect(useGameStore.getState().report('frangfurd')).rejects.toThrow('no run to report')
  })

  it('forgets an earlier refusal when the player tries again', async () => {
    attachBridge()
    vi.mocked(App.StopGame)
      .mockRejectedValueOnce(new Error('first'))
      .mockResolvedValue(running as never)
    await expect(useGameStore.getState().stop('frangfurd')).rejects.toThrow('first')
    await useGameStore.getState().stop('frangfurd')
    expect(useGameStore.getState().stopErrors).toEqual({})
  })
})
