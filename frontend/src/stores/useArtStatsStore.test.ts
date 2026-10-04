import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as App from '../../wailsjs/go/main/App'
import { useArtStatsStore } from './useArtStatsStore'

vi.mock('../../wailsjs/go/main/App')

describe('useArtStatsStore', () => {
  beforeEach(() => {
    useArtStatsStore.setState({ stats: null })
    vi.mocked(App.GetWikiArtStats).mockReset()
  })

  it('reads the pools and the average picture', async () => {
    vi.mocked(App.GetWikiArtStats).mockResolvedValue({ avgBytes: 1000, pools: { Luxemburg: 3 } })
    await useArtStatsStore.getState().load()
    expect(useArtStatsStore.getState().stats).toEqual({ avgBytes: 1000, pools: { Luxemburg: 3 } })
  })

  it('has none without a bridge or an export, and takes null pools as none', async () => {
    vi.mocked(App.GetWikiArtStats).mockRejectedValue('no export')
    await useArtStatsStore.getState().load()
    expect(useArtStatsStore.getState().stats).toBeNull()
    vi.mocked(App.GetWikiArtStats).mockResolvedValue({ avgBytes: 5, pools: null } as never)
    await useArtStatsStore.getState().load()
    expect(useArtStatsStore.getState().stats).toEqual({ avgBytes: 5, pools: {} })
  })
})
