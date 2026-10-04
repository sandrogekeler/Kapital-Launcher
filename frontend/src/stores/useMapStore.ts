import { create } from 'zustand'
import type { MapStatus } from '../types'
import { errMsg, hasWailsBridge } from '../lib/ipc'
import { CheckChapterMap } from '../../wailsjs/go/main/App'

/**
 * The map page's one fact (issue 161): whether the chapter's map answered when
 * Go asked. Only the lazy page imports this store and it empties it on the way
 * out. The address is Go's, the manifest's; the page never sends one.
 */
interface MapStore {
  /** What the last check found; null before the first answer and while a check is in flight. */
  status: MapStatus | null
  /**
   * Asks Go whether the chapter's map answers. `bundledUrl` is the address the
   * bundled manifest gives the chapter, used only with no bridge (the
   * browser-only preview), where nothing can be asked and the frame is left
   * to show for itself whether the address loads. A refusal from Go, which
   * only an unknown chapter earns, is a map that could not be reached.
   */
  check: (chapterId: string, bundledUrl?: string) => Promise<void>
  clear: () => void
}

// An answer that arrives after the page moved on (Try again pressed twice, the
// page closed) is dropped: only the latest check's answer is kept.
let epoch = 0

export const useMapStore = create<MapStore>((set) => ({
  status: null,

  check: async (chapterId, bundledUrl = '') => {
    const mine = ++epoch
    set({ status: null })
    let status: MapStatus
    if (!hasWailsBridge()) {
      status = {
        url: bundledUrl,
        reachable: bundledUrl !== '',
        reason: bundledUrl === '' ? 'no map' : '',
      }
    } else {
      try {
        status = await CheckChapterMap(chapterId)
      } catch (e) {
        status = { url: bundledUrl, reachable: false, reason: errMsg(e) }
      }
    }
    if (mine === epoch) set({ status })
  },

  clear: () => {
    epoch++
    set({ status: null })
  },
}))
