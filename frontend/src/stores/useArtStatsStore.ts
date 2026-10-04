import { create } from 'zustand'
import type { WikiArtStats } from '../types'
import { readOr } from '../lib/ipc'
import { GetWikiArtStats } from '../../wailsjs/go/main/App'

/**
 * What the wiki pictures weigh and how many each chapter has to draw from
 * (issue 172), for the settings screen's estimate of the room a number of
 * pictures takes. Only that screen reads it, so it is a store of its own and
 * stays out of the chapter store, which the launcher's first paint loads. A
 * read: with no bridge, or no export to count, there are no stats and the
 * estimate goes by the number alone.
 */
interface ArtStatsStore {
  stats: WikiArtStats | null
  load: () => Promise<void>
}

export const useArtStatsStore = create<ArtStatsStore>((set) => ({
  stats: null,
  load: async () => {
    const stats = await readOr(GetWikiArtStats, null)
    set({
      stats: stats && typeof stats === 'object' ? { ...stats, pools: stats.pools ?? {} } : null,
    })
  },
}))
