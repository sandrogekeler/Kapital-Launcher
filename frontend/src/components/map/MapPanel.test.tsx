import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import * as Bindings from '../../../wailsjs/go/main/App'
import { useMapStore } from '../../stores/useMapStore'
import { BUNDLED_MANIFEST } from '../../lib/manifest'
import type { MapStatus } from '../../types'
import { MapPanel } from './MapPanel'

vi.mock('../../../wailsjs/go/main/App')

const MAP = 'http://spiral-reminders.tun.ply.gg:1111'
const frangfurd = BUNDLED_MANIFEST.chapters.find((c) => c.id === 'frangfurd')!
const luxemburg = BUNDLED_MANIFEST.chapters.find((c) => c.id === 'luxemburg')!

const answer = (over: Partial<MapStatus> = {}): MapStatus => ({
  url: MAP,
  reachable: true,
  reason: '',
  ...over,
})

function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((r) => (resolve = r))
  return { promise, resolve }
}

describe('MapPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.assign(window, { go: {} })
    useMapStore.getState().clear()
    vi.mocked(Bindings.OpenExternal).mockResolvedValue()
  })
  afterEach(() => {
    cleanup()
    Reflect.deleteProperty(window, 'go')
  })

  it('stays unrevealed until Go answers, then frames the map', async () => {
    const pending = deferred<MapStatus>()
    vi.mocked(Bindings.CheckChapterMap).mockReturnValue(pending.promise as never)
    render(<MapPanel chapter={frangfurd} onClose={() => undefined} />)
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Frangfurd map')
    expect(screen.getByText('Reading.')).toBeInTheDocument()
    expect(screen.queryByTitle('Frangfurd map')).toBeNull()
    expect(screen.queryByText('Map could not be reached')).toBeNull()
    expect(Bindings.CheckChapterMap).toHaveBeenCalledExactlyOnceWith('frangfurd')

    pending.resolve(answer())
    const frame = await screen.findByTitle('Frangfurd map')
    expect(frame).toHaveAttribute('src', MAP)
    expect(screen.queryByText('Reading.')).toBeNull()
    expect(screen.queryByText('Map could not be reached')).toBeNull()
  })

  it('frames it sandboxed to scripts and its own storage, with no referrer', async () => {
    vi.mocked(Bindings.CheckChapterMap).mockResolvedValue(answer())
    render(<MapPanel chapter={frangfurd} onClose={() => undefined} />)
    const frame = await screen.findByTitle('Frangfurd map')
    // Exactly these two tokens: BlueMap needs scripts and its own origin's
    // storage, and nothing may navigate the launcher, open a window, submit or
    // lock the pointer.
    expect(frame.getAttribute('sandbox')?.split(' ').sort()).toEqual([
      'allow-same-origin',
      'allow-scripts',
    ])
    expect(frame).toHaveAttribute('referrerpolicy', 'no-referrer')
    expect(frame).toHaveClass('rounded-md', 'border', 'grow')
  })

  it('offers Open in browser in the header, with the manifest address', async () => {
    vi.mocked(Bindings.CheckChapterMap).mockResolvedValue(answer())
    render(<MapPanel chapter={frangfurd} onClose={() => undefined} />)
    await screen.findByTitle('Frangfurd map')
    fireEvent.click(screen.getByRole('button', { name: 'Open in browser' }))
    expect(Bindings.OpenExternal).toHaveBeenCalledExactlyOnceWith(MAP)
  })

  it('shows what the browser open refused', async () => {
    vi.mocked(Bindings.CheckChapterMap).mockResolvedValue(answer())
    vi.mocked(Bindings.OpenExternal).mockRejectedValue('no default browser')
    render(<MapPanel chapter={frangfurd} onClose={() => undefined} />)
    await screen.findByTitle('Frangfurd map')
    fireEvent.click(screen.getByRole('button', { name: 'Open in browser' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('no default browser')
  })

  it('says the map could not be reached when the address did not answer, and tries again', async () => {
    vi.mocked(Bindings.CheckChapterMap)
      .mockResolvedValueOnce(answer({ reachable: false, reason: 'timed out' }))
      .mockResolvedValueOnce(answer())
    render(<MapPanel chapter={frangfurd} onClose={() => undefined} />)
    expect(await screen.findByRole('heading', { name: 'Map could not be reached' })).toBeVisible()
    expect(screen.getByText('The address did not answer.')).toBeInTheDocument()
    expect(screen.queryByTitle('Frangfurd map')).toBeNull()
    // The way out to the browser is still there, for an address the launcher
    // could not see.
    expect(screen.getByRole('button', { name: 'Open in browser' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(screen.getByRole('status')).toHaveTextContent('Checking the map.')
    expect(await screen.findByTitle('Frangfurd map')).toHaveAttribute('src', MAP)
    expect(Bindings.CheckChapterMap).toHaveBeenCalledTimes(2)
    expect(Bindings.CheckChapterMap).toHaveBeenLastCalledWith('frangfurd')
  })

  it('says there is no map yet for a chapter without one, with no browser button, and still tries again', async () => {
    vi.mocked(Bindings.CheckChapterMap).mockResolvedValue(
      answer({ url: '', reachable: false, reason: 'no map' }),
    )
    render(<MapPanel chapter={luxemburg} onClose={() => undefined} />)
    expect(await screen.findByRole('heading', { name: 'Map could not be reached' })).toBeVisible()
    expect(screen.getByText('There is no map for Luxemburg yet.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Open in browser' })).toBeNull()
    expect(document.querySelector('iframe')).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(Bindings.CheckChapterMap).toHaveBeenCalledTimes(2))
    expect(await screen.findByText('There is no map for Luxemburg yet.')).toBeInTheDocument()
  })

  it('never frames an address Go did not give, even from a reachable answer with none', async () => {
    vi.mocked(Bindings.CheckChapterMap).mockResolvedValue(answer({ url: '' }))
    render(<MapPanel chapter={luxemburg} onClose={() => undefined} />)
    expect(await screen.findByRole('heading', { name: 'Map could not be reached' })).toBeVisible()
    expect(document.querySelector('iframe')).toBeNull()
  })

  it('closes on Back and on Escape, and empties the check as it goes', async () => {
    vi.mocked(Bindings.CheckChapterMap).mockResolvedValue(answer())
    const onClose = vi.fn()
    const { unmount } = render(<MapPanel chapter={frangfurd} onClose={onClose} />)
    await screen.findByTitle('Frangfurd map')
    fireEvent.click(screen.getByRole('button', { name: 'Back' }))
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(2)
    unmount()
    expect(useMapStore.getState().status).toBeNull()
  })
})
