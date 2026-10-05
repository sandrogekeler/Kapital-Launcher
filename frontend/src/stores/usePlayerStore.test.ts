import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as Bindings from '../../wailsjs/go/main/App'
import { usePlayerStore } from './usePlayerStore'

vi.mock('../../wailsjs/go/main/App')

const FOUND = {
  name: 'Snadrochka',
  uuid: '069a79f4-44e9-4726-a5be-fca90e38aaf5',
  faceSrc: '/mojang-face/069a79f444e94726a5befca90e38aaf5.png?v=1',
  status: 'found',
} as const

/** A promise settled by hand, for an answer that arrives late. */
function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((r) => (resolve = r))
  return { promise, resolve }
}

describe('usePlayerStore', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    Object.assign(window, { go: {} })
    usePlayerStore.getState().clear()
  })
  afterEach(() => Reflect.deleteProperty(window, 'go'))

  it("keeps Go's answer for the trimmed name, and is empty while it is on its way", async () => {
    const answer = deferred<typeof FOUND>()
    vi.mocked(Bindings.GetPlayerProfile).mockReturnValue(answer.promise as never)
    const pending = usePlayerStore.getState().check(' Snadrochka ')
    expect(usePlayerStore.getState().profile).toBeNull()
    answer.resolve(FOUND)
    await pending
    expect(Bindings.GetPlayerProfile).toHaveBeenCalledExactlyOnceWith('Snadrochka')
    expect(usePlayerStore.getState().profile).toEqual(FOUND)
  })

  it('asks nothing for an empty name', async () => {
    await usePlayerStore.getState().check('  ')
    expect(Bindings.GetPlayerProfile).not.toHaveBeenCalled()
    expect(usePlayerStore.getState().profile).toBeNull()
  })

  it('drops an answer that arrives after a newer check or the page leaving', async () => {
    const slow = deferred<typeof FOUND>()
    vi.mocked(Bindings.GetPlayerProfile)
      .mockReturnValueOnce(slow.promise as never)
      .mockResolvedValueOnce({ ...FOUND, name: 'Notch' } as never)
    const first = usePlayerStore.getState().check('Snadrochka')
    await usePlayerStore.getState().check('Notch')
    slow.resolve(FOUND)
    await first
    expect(usePlayerStore.getState().profile?.name).toBe('Notch')
    usePlayerStore.getState().clear()
    expect(usePlayerStore.getState().profile).toBeNull()
  })

  it('degrades to unknown with no bridge, on a rejection and on an empty answer', async () => {
    // The real binding throws, synchronously, with no window.go.
    Reflect.deleteProperty(window, 'go')
    vi.mocked(Bindings.GetPlayerProfile).mockImplementationOnce(() => {
      throw new TypeError("Cannot read properties of undefined (reading 'main')")
    })
    await usePlayerStore.getState().check('Snadrochka')
    expect(usePlayerStore.getState().profile?.status).toBe('unknown')

    Object.assign(window, { go: {} })
    vi.mocked(Bindings.GetPlayerProfile).mockRejectedValueOnce('boom')
    await usePlayerStore.getState().check('Snadrochka')
    expect(usePlayerStore.getState().profile?.status).toBe('unknown')

    vi.mocked(Bindings.GetPlayerProfile).mockResolvedValueOnce(undefined as never)
    await usePlayerStore.getState().check('Snadrochka')
    expect(usePlayerStore.getState().profile?.status).toBe('unknown')
  })

  it('copies through Go, and says so when there is no bridge or Go refuses', async () => {
    vi.mocked(Bindings.CopyPlayerUUID).mockResolvedValueOnce()
    await usePlayerStore.getState().copyUuid()
    expect(Bindings.CopyPlayerUUID).toHaveBeenCalledOnce()

    vi.mocked(Bindings.CopyPlayerUUID).mockRejectedValueOnce('no player profile to copy')
    await expect(usePlayerStore.getState().copyUuid()).rejects.toBe('no player profile to copy')

    Reflect.deleteProperty(window, 'go')
    await expect(usePlayerStore.getState().copyUuid()).rejects.toThrow(/app window/)
  })
})
