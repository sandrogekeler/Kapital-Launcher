import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as Bindings from '../../wailsjs/go/main/App'
import { useMapStore } from './useMapStore'

vi.mock('../../wailsjs/go/main/App')

const MAP = 'http://spiral-reminders.tun.ply.gg:1111'

/** A promise settled by hand, for an answer that arrives late. */
function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((r) => (resolve = r))
  return { promise, resolve }
}

describe('useMapStore', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.assign(window, { go: {} })
    useMapStore.getState().clear()
  })
  afterEach(() => Reflect.deleteProperty(window, 'go'))

  it("keeps Go's answer, and is empty while the check is on its way", async () => {
    const answer = deferred<{ url: string; reachable: boolean; reason: string }>()
    vi.mocked(Bindings.CheckChapterMap).mockReturnValue(answer.promise as never)
    const pending = useMapStore.getState().check('frangfurd', MAP)
    expect(useMapStore.getState().status).toBeNull()
    answer.resolve({ url: MAP, reachable: true, reason: '' })
    await pending
    expect(Bindings.CheckChapterMap).toHaveBeenCalledExactlyOnceWith('frangfurd')
    expect(useMapStore.getState().status).toEqual({ url: MAP, reachable: true, reason: '' })
  })

  it('asks again from Try again and clears the old answer first', async () => {
    vi.mocked(Bindings.CheckChapterMap)
      .mockResolvedValueOnce({ url: MAP, reachable: false, reason: 'timed out' } as never)
      .mockResolvedValueOnce({ url: MAP, reachable: true, reason: '' } as never)
    await useMapStore.getState().check('frangfurd', MAP)
    expect(useMapStore.getState().status?.reachable).toBe(false)
    const again = useMapStore.getState().check('frangfurd', MAP)
    expect(useMapStore.getState().status).toBeNull()
    await again
    expect(useMapStore.getState().status?.reachable).toBe(true)
  })

  it('drops an answer that arrives after a later check or after the page left', async () => {
    const slow = deferred<{ url: string; reachable: boolean; reason: string }>()
    vi.mocked(Bindings.CheckChapterMap)
      .mockReturnValueOnce(slow.promise as never)
      .mockResolvedValueOnce({ url: MAP, reachable: true, reason: '' } as never)
    const first = useMapStore.getState().check('frangfurd', MAP)
    await useMapStore.getState().check('frangfurd', MAP)
    slow.resolve({ url: MAP, reachable: false, reason: 'timed out' })
    await first
    expect(useMapStore.getState().status?.reachable).toBe(true)

    const late = deferred<{ url: string; reachable: boolean; reason: string }>()
    vi.mocked(Bindings.CheckChapterMap).mockReturnValueOnce(late.promise as never)
    const left = useMapStore.getState().check('frangfurd', MAP)
    useMapStore.getState().clear()
    late.resolve({ url: MAP, reachable: true, reason: '' })
    await left
    expect(useMapStore.getState().status).toBeNull()
  })

  it('takes a refusal from Go as a map that could not be reached', async () => {
    vi.mocked(Bindings.CheckChapterMap).mockRejectedValue('no chapter "atlantis"')
    await useMapStore.getState().check('atlantis')
    expect(useMapStore.getState().status).toEqual({
      url: '',
      reachable: false,
      reason: 'no chapter "atlantis"',
    })
  })

  it('has nothing to ask without a bridge: the bundled address stands, or there is none', async () => {
    Reflect.deleteProperty(window, 'go')
    await useMapStore.getState().check('frangfurd', MAP)
    expect(Bindings.CheckChapterMap).not.toHaveBeenCalled()
    expect(useMapStore.getState().status).toEqual({ url: MAP, reachable: true, reason: '' })
    await useMapStore.getState().check('luxemburg')
    expect(useMapStore.getState().status).toEqual({ url: '', reachable: false, reason: 'no map' })
  })
})
