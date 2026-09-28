import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as App from '../../wailsjs/go/main/App'
import * as Runtime from '../../wailsjs/runtime/runtime'
import { EVENT_SERVER_STATUS, selectStatus, useServerStore } from './useServerStore'
import type { ServerStatus } from '../types'

vi.mock('../../wailsjs/go/main/App')
vi.mock('../../wailsjs/runtime/runtime')

const attachBridge = () => Object.assign(window, { go: {} })
const detachBridge = () => {
  delete (window as unknown as { go?: unknown }).go
}

const online: ServerStatus = {
  chapterId: 'lichdenstein',
  checked: true,
  online: true,
  players: 3,
  max: 20,
  version: 'Paper 1.20.6',
  motd: 'Lichdenstein',
  latencyMs: 41,
  checkedAt: '2026-09-28T12:00:00Z',
}

describe('useServerStore', () => {
  beforeEach(() => {
    useServerStore.setState({ statuses: {}, checking: null, error: null })
    vi.mocked(App.GetServerStatus).mockReset()
    vi.mocked(Runtime.EventsOn).mockReset()
    vi.mocked(Runtime.EventsOff).mockReset()
  })
  afterEach(detachBridge)

  it('files a status under its chapter and drops one without', () => {
    useServerStore.getState().receive(online)
    useServerStore.getState().receive({ ...online, chapterId: '' })
    expect(selectStatus('lichdenstein')(useServerStore.getState())).toEqual(online)
    expect(Object.keys(useServerStore.getState().statuses)).toEqual(['lichdenstein'])
  })

  it('checks on demand and records a real failure', async () => {
    attachBridge()
    vi.mocked(App.GetServerStatus).mockResolvedValue(online)
    await useServerStore.getState().check('lichdenstein')
    expect(useServerStore.getState().statuses.lichdenstein?.online).toBe(true)
    expect(useServerStore.getState().checking).toBeNull()

    vi.mocked(App.GetServerStatus).mockRejectedValue(new Error('no chapter "x"'))
    await useServerStore.getState().check('x')
    expect(useServerStore.getState().error).toContain('no chapter')
  })

  it('stays silent without a bridge', async () => {
    vi.mocked(App.GetServerStatus).mockImplementation(() => {
      throw new TypeError('no bridge')
    })
    await useServerStore.getState().check('lichdenstein')
    expect(useServerStore.getState().error).toBeNull()
    expect(useServerStore.getState().statuses).toEqual({})
    expect(useServerStore.getState().listen()).toBeTypeOf('function')
    expect(Runtime.EventsOn).not.toHaveBeenCalled()
  })

  it('listens to the runtime event with a bridge and unsubscribes', () => {
    attachBridge()
    let handler: ((s: ServerStatus) => void) | undefined
    vi.mocked(Runtime.EventsOn).mockImplementation((_name, cb) => {
      handler = cb as (s: ServerStatus) => void
      return () => undefined
    })
    const off = useServerStore.getState().listen()
    expect(Runtime.EventsOn).toHaveBeenCalledWith(EVENT_SERVER_STATUS, expect.any(Function))
    handler?.({ ...online, chapterId: 'frangfurd', online: false })
    expect(useServerStore.getState().statuses.frangfurd?.online).toBe(false)
    off()
    expect(Runtime.EventsOff).toHaveBeenCalledWith(EVENT_SERVER_STATUS)
  })
})
