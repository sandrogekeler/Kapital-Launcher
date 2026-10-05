import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as Bindings from '../../wailsjs/go/main/App'
import { useModStore } from './useModStore'
import type { ChapterMods } from '../types'

vi.mock('../../wailsjs/go/main/App')

const dh = 'DistantHorizons-3.3.3-1.21.1-fabric-neoforge.jar'
const jade = 'Jade-1.21.1-NeoForge-15.1.jar'

const view = (off: string[] = []): ChapterMods => ({
  chapterId: 'frangfurd',
  mods: [dh, jade].map((name) => ({ name, disabled: off.includes(name), size: 10 })),
  toggles: [
    {
      name: 'Distant Horizons',
      jarPrefix: 'DistantHorizons-',
      jars: [dh],
      disabled: off.includes(dh),
      blocked: false,
    },
  ],
  running: false,
})

describe('useModStore', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.assign(window, { go: {} })
    useModStore.setState({ byChapter: {}, unavailable: false, readError: {}, writing: null })
  })
  afterEach(() => Reflect.deleteProperty(window, 'go'))

  it('reads a chapter, and says so when there is no bridge', async () => {
    vi.mocked(Bindings.GetChapterMods).mockResolvedValue(view() as never)
    await useModStore.getState().load('frangfurd')
    expect(useModStore.getState().byChapter.frangfurd).toEqual(view())
    expect(useModStore.getState().unavailable).toBe(false)
    Reflect.deleteProperty(window, 'go')
    await useModStore.getState().load('frangfurd')
    expect(useModStore.getState().unavailable).toBe(true)
  })

  it('keeps the reason a read was refused, by chapter, and clears it on the next good read', async () => {
    vi.mocked(Bindings.GetChapterMods).mockRejectedValue('Frangfurd is not installed')
    await useModStore.getState().load('frangfurd')
    expect(useModStore.getState().readError.frangfurd).toBe('Frangfurd is not installed')
    expect(useModStore.getState().byChapter.frangfurd).toBeUndefined()
    vi.mocked(Bindings.GetChapterMods).mockResolvedValue(view() as never)
    await useModStore.getState().load('frangfurd')
    expect(useModStore.getState().readError.frangfurd).toBeUndefined()
  })

  it('shows a switch at once and keeps what Go answers', async () => {
    useModStore.setState({ byChapter: { frangfurd: view() } })
    let release!: (v: ChapterMods) => void
    vi.mocked(Bindings.SetModsDisabled).mockReturnValue(
      new Promise((r) => (release = r as never)) as never,
    )
    const write = useModStore.getState().setJars('frangfurd', [dh], true)
    // Before Go has answered: drawn as it will be, a write in flight.
    expect(useModStore.getState().byChapter.frangfurd?.toggles[0]?.disabled).toBe(true)
    expect(useModStore.getState().writing).toBe('frangfurd')
    release(view([dh]))
    await write
    expect(Bindings.SetModsDisabled).toHaveBeenCalledWith('frangfurd', [dh])
    expect(useModStore.getState()).toMatchObject({ writing: null })
    expect(useModStore.getState().byChapter.frangfurd).toEqual(view([dh]))
  })

  it('sends the whole list: what was off stays off', async () => {
    useModStore.setState({ byChapter: { frangfurd: view([dh]) } })
    vi.mocked(Bindings.SetModsDisabled).mockResolvedValue(view([dh, jade]) as never)
    await useModStore.getState().setJars('frangfurd', [jade], true)
    expect(Bindings.SetModsDisabled).toHaveBeenCalledWith('frangfurd', [dh, jade])
    vi.mocked(Bindings.SetModsDisabled).mockResolvedValue(view() as never)
    useModStore.setState({ byChapter: { frangfurd: view([dh, jade]) } })
    await useModStore.getState().setJars('frangfurd', [dh, jade], false)
    expect(Bindings.SetModsDisabled).toHaveBeenLastCalledWith('frangfurd', [])
  })

  it('puts the view back and rethrows when a real backend refuses, and reads again', async () => {
    useModStore.setState({ byChapter: { frangfurd: view() } })
    vi.mocked(Bindings.SetModsDisabled).mockRejectedValue(
      'Frangfurd is starting or running; close the game first',
    )
    vi.mocked(Bindings.GetChapterMods).mockResolvedValue(view() as never)
    await expect(useModStore.getState().setJars('frangfurd', [dh], true)).rejects.toBe(
      'Frangfurd is starting or running; close the game first',
    )
    expect(useModStore.getState().byChapter.frangfurd).toEqual(view())
    expect(useModStore.getState().writing).toBeNull()
    expect(Bindings.GetChapterMods).toHaveBeenCalledWith('frangfurd')
  })

  it('lets a write stand with no bridge, and does nothing before the first read', async () => {
    await useModStore.getState().setJars('frangfurd', [dh], true)
    expect(Bindings.SetModsDisabled).not.toHaveBeenCalled()
    expect(useModStore.getState().byChapter.frangfurd).toBeUndefined()
    useModStore.setState({ byChapter: { frangfurd: view() } })
    Reflect.deleteProperty(window, 'go')
    await useModStore.getState().setJars('frangfurd', [dh], true)
    expect(Bindings.SetModsDisabled).not.toHaveBeenCalled()
    expect(useModStore.getState().byChapter.frangfurd?.mods[0]?.disabled).toBe(true)
    expect(useModStore.getState().writing).toBeNull()
  })
})
