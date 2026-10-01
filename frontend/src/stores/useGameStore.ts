import { create } from 'zustand'
import type { GamePhase, GameState } from '../types'
import { hasWailsBridge, readOr } from '../lib/ipc'
import { GetGameStates } from '../../wailsjs/go/main/App'
import { EventsOff, EventsOn } from '../../wailsjs/runtime/runtime'

/** The event Go emits on every phase change. Same string as services.EventGameState. */
export const EVENT_GAME_STATE = 'game:state'

/** The phases in which the game is starting, running or closing. */
const ACTIVE: readonly GamePhase[] = [
  'starting',
  'mods',
  'window',
  'resources',
  'running',
  'stopping',
]

/** Whether a game in this phase is in progress, so Play and Install wait. */
export const isActive = (phase: GamePhase | undefined): boolean =>
  phase !== undefined && ACTIVE.includes(phase)

/**
 * Where each chapter's game is, from Play to its end. Go follows the game's
 * process and log and emits a `game:state` per phase; this store listens and
 * holds the latest per chapter, and `load` reads the state now. Nothing here
 * polls: the cadence is the game's.
 */
interface GameStore {
  states: Record<string, GameState>
  listen: () => () => void
  load: () => Promise<void>
  receive: (state: GameState) => void
}

export const useGameStore = create<GameStore>((set, get) => ({
  states: {},

  // Subscribes for the app's lifetime; returns the unsubscribe for tests and
  // for a StrictMode double mount. Without a bridge there is no runtime to
  // subscribe to, and the browser-only preview never hears an update.
  listen: () => {
    if (!hasWailsBridge()) return () => undefined
    EventsOn(EVENT_GAME_STATE, (state: GameState) => get().receive(state))
    return () => EventsOff(EVENT_GAME_STATE)
  },

  // Reads every chapter's state once, for a window that opens after a game
  // began. Without a bridge it degrades to nothing known.
  load: async () => {
    // The bindings type the phase as a plain string; the Go constants are
    // the GamePhase union.
    const states = (await readOr(GetGameStates, [])) as GameState[]
    for (const state of states) get().receive(state)
  },

  // A payload names its chapter and is filed under it. A payload without one
  // is dropped: nothing can be said about a game that is not named.
  receive: (state) => {
    if (!state?.chapterId) return
    set((s) => ({ states: { ...s.states, [state.chapterId]: state } }))
  },
}))

/** The game state for one chapter, or undefined before anything is known. */
export const selectGame = (chapterId: string) => (s: GameStore) => s.states[chapterId]
