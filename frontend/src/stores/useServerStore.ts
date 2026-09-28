import { create } from 'zustand'
import type { ServerStatus } from '../types'
import { errMsg, hasWailsBridge } from '../lib/ipc'
import { GetServerStatus } from '../../wailsjs/go/main/App'
import { EventsOff, EventsOn } from '../../wailsjs/runtime/runtime'

/** The event Go emits after every ping. Same string as services.EventServerStatus. */
export const EVENT_SERVER_STATUS = 'server:status'

/**
 * Server status per chapter. Go pings on a ticker and emits an event per
 * result; this store listens and holds the latest per chapter, and `check`
 * asks for one now. Nothing here polls: the cadence is Go's.
 */
interface ServerStore {
  statuses: Record<string, ServerStatus>
  checking: string | null
  error: string | null
  listen: () => () => void
  check: (chapterId: string) => Promise<void>
  receive: (status: ServerStatus) => void
}

export const useServerStore = create<ServerStore>((set, get) => ({
  statuses: {},
  checking: null,
  error: null,

  // Subscribes for the app's lifetime; returns the unsubscribe for tests and
  // for a StrictMode double mount. Without a bridge there is no runtime to
  // subscribe to, and the browser-only preview simply never hears an update.
  listen: () => {
    if (!hasWailsBridge()) return () => undefined
    EventsOn(EVENT_SERVER_STATUS, (status: ServerStatus) => get().receive(status))
    return () => EventsOff(EVENT_SERVER_STATUS)
  },

  // A payload names its chapter and is filed under it. A payload without one
  // is dropped: nothing can be said about a server that is not named.
  receive: (status) => {
    if (!status?.chapterId) return
    set((s) => ({ statuses: { ...s.statuses, [status.chapterId]: status } }))
  },

  check: async (chapterId) => {
    set({ checking: chapterId, error: null })
    try {
      get().receive(await GetServerStatus(chapterId))
    } catch (e) {
      // No bridge is the preview: leave the chapter unchecked and say nothing.
      if (hasWailsBridge()) set({ error: errMsg(e) })
    } finally {
      set({ checking: null })
    }
  },
}))

/** The status for one chapter, or undefined before the first ping. */
export const selectStatus = (chapterId: string) => (s: ServerStore) => s.statuses[chapterId]
