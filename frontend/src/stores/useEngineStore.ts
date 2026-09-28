import { create } from 'zustand'
import type { EngineInfo } from '../types'
import { errMsg, readOr } from '../lib/ipc'
import { GetEngine, LaunchChapter, RefreshEngine } from '../../wailsjs/go/main/App'

/**
 * What is known about Prism, and the launch action. The engine is the one
 * thing the app cannot work without, so its absence is a state the UI shows
 * rather than an error it swallows.
 */
interface EngineStore {
  engine: EngineInfo | null
  launching: string | null
  error: string | null
  load: () => Promise<void>
  refresh: () => Promise<void>
  launch: (chapterId: string) => Promise<void>
  clearError: () => void
}

export const useEngineStore = create<EngineStore>((set) => ({
  engine: null,
  launching: null,
  error: null,

  load: async () => {
    set({ engine: await readOr(GetEngine, null) })
  },

  refresh: async () => {
    try {
      set({ engine: await RefreshEngine(), error: null })
    } catch (e) {
      set({ error: errMsg(e) })
    }
  },

  // A launch is a write: with no bridge nothing was going to start, so the
  // rejection is shown as what it is rather than kept optimistic.
  launch: async (chapterId) => {
    set({ launching: chapterId, error: null })
    try {
      await LaunchChapter(chapterId)
    } catch (e) {
      set({ error: errMsg(e) })
    } finally {
      set({ launching: null })
    }
  },

  clearError: () => set({ error: null }),
}))
