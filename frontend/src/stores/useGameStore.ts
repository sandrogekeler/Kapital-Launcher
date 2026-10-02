import { create } from 'zustand'
import type { GamePhase, GameState, RunReport } from '../types'
import { errMsg, hasWailsBridge, readOr } from '../lib/ipc'
import { GetGameStates, GetRunReport, ShowPrismConsole, StopGame } from '../../wailsjs/go/main/App'
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

const without = (errors: Record<string, string>, chapterId: string) =>
  Object.fromEntries(Object.entries(errors).filter(([id]) => id !== chapterId))

/**
 * Where each chapter's game is, from Play to its end. Go follows the game's
 * process and log and emits a `game:state` per phase; this store listens and
 * holds the latest per chapter, and `load` reads the state now. Nothing here
 * polls: the cadence is the game's.
 */
interface GameStore {
  states: Record<string, GameState>
  /** Why the last Stop of each chapter was refused, until the game next changes. */
  stopErrors: Record<string, string>
  listen: () => () => void
  load: () => Promise<void>
  receive: (state: GameState) => void
  /** The chapter's run report, read from Go; null without a bridge. A refusal is rethrown. */
  report: (chapterId: string) => Promise<RunReport | null>
  /** Ends the chapter's run at once. A write: a refusal is recorded and rethrown. */
  stop: (chapterId: string) => Promise<void>
  /** Shows the Prism console Go hid; false when none is left to show. A refusal is rethrown. */
  showConsole: (chapterId: string) => Promise<boolean>
}

export const useGameStore = create<GameStore>((set, get) => ({
  states: {},
  stopErrors: {},

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
    // The game has moved on, so a refusal to stop it is stale.
    set((s) => ({
      states: { ...s.states, [state.chapterId]: state },
      stopErrors: without(s.stopErrors, state.chapterId),
    }))
  },

  // A write like a launch: with no bridge nothing was going to stop, so the
  // rejection is shown as what it is. The answer is not filed: the run's end
  // arrives as a game:state event, and an answer that crossed a newer event
  // would overwrite it.
  stop: async (chapterId) => {
    set((s) => ({ stopErrors: without(s.stopErrors, chapterId) }))
    try {
      await StopGame(chapterId)
    } catch (e) {
      set((s) => ({ stopErrors: { ...s.stopErrors, [chapterId]: errMsg(e) } }))
      throw e
    }
  },

  // The run's report, read from Go when the panel opens and kept nowhere: it
  // holds a stretch of the game's log, so it lives in the panel that shows it
  // and goes with it. Without a bridge there is no run to report on, and null
  // says so; a real backend's refusal rethrows for the panel to show.
  report: async (chapterId) => {
    if (!hasWailsBridge()) return null
    return (await GetRunReport(chapterId)) as RunReport
  },

  // Only a report that says the console is there offers this, and a report
  // needs the bridge, so without one there is nothing to show.
  showConsole: async (chapterId) => (hasWailsBridge() ? ShowPrismConsole(chapterId) : false),
}))

/** The game state for one chapter, or undefined before anything is known. */
export const selectGame = (chapterId: string) => (s: GameStore) => s.states[chapterId]
