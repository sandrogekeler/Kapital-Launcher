import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as App from '../../wailsjs/go/main/App'
import * as Runtime from '../../wailsjs/runtime/runtime'
import { EVENT_GAME_STATE, isActive, selectGame, selectSplash, useGameStore } from './useGameStore'
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
    useGameStore.setState({ states: {}, error: null })
    vi.mocked(App.GetGameStates).mockReset()
    vi.mocked(App.LeaveSplash).mockReset()
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

  it('finds the chapter whose splash is up', () => {
    useGameStore.getState().receive(running)
    expect(selectSplash(useGameStore.getState())).toBeUndefined()
    useGameStore.getState().receive({ ...running, chapterId: 'lichdenstein', splash: true })
    expect(selectSplash(useGameStore.getState())?.chapterId).toBe('lichdenstein')
    useGameStore.getState().receive({ ...running, chapterId: 'lichdenstein', splash: false })
    expect(selectSplash(useGameStore.getState())).toBeUndefined()
  })

  describe('leaveSplash', () => {
    it('asks Go to give the window back', async () => {
      attachBridge()
      vi.mocked(App.LeaveSplash).mockResolvedValue()
      await useGameStore.getState().leaveSplash()
      expect(App.LeaveSplash).toHaveBeenCalledOnce()
      expect(useGameStore.getState().error).toBeNull()
    })

    it('does nothing without a bridge', async () => {
      await useGameStore.getState().leaveSplash()
      expect(App.LeaveSplash).not.toHaveBeenCalled()
      expect(useGameStore.getState().error).toBeNull()
    })

    it('records a rejection for the card and clears it on the next try', async () => {
      attachBridge()
      vi.mocked(App.LeaveSplash).mockRejectedValueOnce('no window').mockResolvedValueOnce()
      await useGameStore.getState().leaveSplash()
      expect(useGameStore.getState().error).toBe('no window')
      await useGameStore.getState().leaveSplash()
      expect(useGameStore.getState().error).toBeNull()
    })
  })
})
