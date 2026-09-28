import { create } from 'zustand'
import type { EngineInfo, InstanceReport } from '../types'
import { errMsg, readOr } from '../lib/ipc'
import { GetEngine, GetInstances, LaunchChapter, RefreshEngine } from '../../wailsjs/go/main/App'

/**
 * What is known about Prism, which chapters' instances it has, and the launch
 * action. The engine is the one thing the app cannot work without, so its
 * absence is a state the UI shows rather than an error it swallows.
 */
interface EngineStore {
  engine: EngineInfo | null
  instances: InstanceReport | null
  launching: string | null
  error: string | null
  load: () => Promise<void>
  loadInstances: () => Promise<void>
  refresh: () => Promise<void>
  launch: (chapterId: string) => Promise<void>
  clearError: () => void
}

export const useEngineStore = create<EngineStore>((set, get) => ({
  engine: null,
  instances: null,
  launching: null,
  error: null,

  load: async () => {
    set({ engine: await readOr(GetEngine, null) })
    await get().loadInstances()
  },

  // A read of the disk as it is now: instances appear when the user imports
  // one in Prism, so App calls this again when the window regains focus.
  loadInstances: async () => {
    set({ instances: await readOr(GetInstances, null) })
  },

  refresh: async () => {
    try {
      set({ engine: await RefreshEngine(), error: null })
    } catch (e) {
      set({ error: errMsg(e) })
      return
    }
    await get().loadInstances()
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

/** Whether the chapter's instance exists: true, false, or undefined for unknown. */
export const selectInstalled = (chapterId: string) => (s: EngineStore) =>
  s.instances?.present[chapterId]
