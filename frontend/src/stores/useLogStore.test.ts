import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as Bindings from '../../wailsjs/go/main/App'
import * as Runtime from '../../wailsjs/runtime/runtime'
import { useLogStore } from './useLogStore'
import type { RunLog, RunLogText } from '../types'

vi.mock('../../wailsjs/go/main/App')
vi.mock('../../wailsjs/runtime/runtime')

const file: RunLog = {
  kind: 'log',
  name: 'latest.log',
  modifiedAt: '2026-10-03T15:00:00Z',
  size: 10,
  crashed: false,
}
const other: RunLog = { ...file, name: '2026-10-02-1.log.gz' }

const chunk = (over: Partial<RunLogText> = {}): RunLogText => ({
  kind: 'log',
  name: 'latest.log',
  text: 'b\n',
  offset: 2,
  size: 4,
  lines: 1,
  truncated: true,
  ...over,
})

/** A promise settled by hand, for an answer that arrives late. */
function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((r) => (resolve = r))
  return { promise, resolve }
}

describe('useLogStore', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.assign(window, { go: {} })
    useLogStore.getState().clear()
  })
  afterEach(() => Reflect.deleteProperty(window, 'go'))

  it('reads the list, and without a bridge says nothing can be read', async () => {
    vi.mocked(Bindings.GetRunLogs).mockResolvedValue([file] as never)
    await useLogStore.getState().load('frangfurd')
    expect(useLogStore.getState().logs).toEqual([file])
    Reflect.deleteProperty(window, 'go')
    await useLogStore.getState().load('frangfurd')
    expect(useLogStore.getState()).toMatchObject({ logs: null, unavailable: true })
  })

  it('records a refused list and a refused file, each in its own place', async () => {
    vi.mocked(Bindings.GetRunLogs).mockRejectedValue('no list')
    await useLogStore.getState().load('frangfurd')
    expect(useLogStore.getState().listError).toBe('no list')
    vi.mocked(Bindings.ReadRunLog).mockRejectedValue(new Error('no file'))
    await useLogStore.getState().open('frangfurd', file)
    expect(useLogStore.getState()).toMatchObject({
      readError: 'no file',
      reading: false,
      opened: null,
    })
    expect(useLogStore.getState().selected).toEqual(file)
  })

  it('keeps only the answer of the file asked for last', async () => {
    const slow = deferred<RunLogText>()
    vi.mocked(Bindings.ReadRunLog)
      .mockReturnValueOnce(slow.promise as never)
      .mockResolvedValueOnce(chunk({ name: other.name, text: 'second\n' }) as never)
    const first = useLogStore.getState().open('frangfurd', file)
    await useLogStore.getState().open('frangfurd', other)
    slow.resolve(chunk({ text: 'first\n' }))
    await first
    expect(useLogStore.getState().opened?.text).toBe('second\n')
    expect(useLogStore.getState().selected).toEqual(other)
  })

  it('drops an answer that arrives after the page was cleared', async () => {
    const slow = deferred<RunLogText>()
    vi.mocked(Bindings.ReadRunLog).mockReturnValueOnce(slow.promise as never)
    const pending = useLogStore.getState().open('frangfurd', file)
    useLogStore.getState().clear()
    slow.resolve(chunk())
    await pending
    expect(useLogStore.getState()).toMatchObject({ opened: null, selected: null })
  })

  it('puts the earlier chunk in front and sums the lines, and ignores a file that is whole', async () => {
    vi.mocked(Bindings.ReadRunLog)
      .mockResolvedValueOnce(chunk({ text: 'c\nd\n', offset: 4, lines: 2 }) as never)
      .mockResolvedValueOnce(
        chunk({ text: 'a\nb\n', offset: 0, lines: 2, truncated: false }) as never,
      )
    await useLogStore.getState().open('frangfurd', file)
    await useLogStore.getState().loadEarlier('frangfurd')
    expect(vi.mocked(Bindings.ReadRunLog).mock.calls[1]).toEqual([
      'frangfurd',
      'log',
      'latest.log',
      4,
    ])
    expect(useLogStore.getState().opened).toMatchObject({
      text: 'a\nb\nc\nd\n',
      lines: 4,
      offset: 0,
      truncated: false,
    })
    await useLogStore.getState().loadEarlier('frangfurd')
    expect(Bindings.ReadRunLog).toHaveBeenCalledTimes(2)
  })

  it('keeps what is shown when the earlier chunk is refused', async () => {
    vi.mocked(Bindings.ReadRunLog)
      .mockResolvedValueOnce(chunk() as never)
      .mockRejectedValueOnce('gone')
    await useLogStore.getState().open('frangfurd', file)
    await useLogStore.getState().loadEarlier('frangfurd')
    expect(useLogStore.getState()).toMatchObject({ readError: 'gone', reading: false })
    expect(useLogStore.getState().opened?.text).toBe('b\n')
  })

  it('copies what is shown through the runtime, and rejects when it could not', async () => {
    vi.mocked(Bindings.ReadRunLog).mockResolvedValue(chunk({ text: 'shown\n' }) as never)
    await useLogStore.getState().open('frangfurd', file)
    vi.mocked(Runtime.ClipboardSetText).mockResolvedValueOnce(true).mockResolvedValueOnce(false)
    await useLogStore.getState().copy()
    expect(Runtime.ClipboardSetText).toHaveBeenCalledExactlyOnceWith('shown\n')
    await expect(useLogStore.getState().copy()).rejects.toThrow('could not be copied')
    Reflect.deleteProperty(window, 'go')
    await expect(useLogStore.getState().copy()).rejects.toThrow('app window')
  })

  it('has nothing to copy before a file is open', async () => {
    await useLogStore.getState().copy()
    expect(Runtime.ClipboardSetText).not.toHaveBeenCalled()
  })
})
