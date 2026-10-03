import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import * as App from '../../../wailsjs/go/main/App'
import { BUNDLED_MANIFEST } from '../../lib/manifest'
import { useChapterStore } from '../../stores/useChapterStore'
import { usePreviewStore } from '../../stores/usePreviewStore'
import { DEFAULT_SETTINGS, useSettingsStore } from '../../stores/useSettingsStore'
import type { PreviewSituation } from '../../types'
import { PreviewSection } from './PreviewSection'

vi.mock('../../../wailsjs/go/main/App')
vi.mock('../../../wailsjs/runtime/runtime')

const list: PreviewSituation[] = [
  {
    id: 'crashed',
    label: 'Crashed, with a crash report',
    scope: 'chapter',
    card: true,
    playsInstall: false,
  },
  {
    id: 'running',
    label: 'Running, to try Stop',
    scope: 'chapter',
    card: false,
    playsInstall: false,
  },
  { id: 'prism-missing', label: 'Prism missing', scope: 'prism', card: false, playsInstall: false },
]

describe('PreviewSection', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    Object.assign(window, { go: {} })
    useChapterStore.setState({ manifest: BUNDLED_MANIFEST, selectedId: 'frangfurd' })
    useSettingsStore.setState({ settings: DEFAULT_SETTINGS, loaded: true, error: null })
    usePreviewStore.setState({ situations: [] })
    vi.mocked(App.GetPreviewSituations).mockResolvedValue(list as never)
    vi.mocked(App.StartPreview).mockResolvedValue({ cardSkipped: false })
    vi.mocked(App.ClearPreviews).mockResolvedValue()
    vi.mocked(App.GetEngine).mockResolvedValue({
      found: true,
      executable: '',
      version: '',
      root: '',
      source: 'path',
    })
    vi.mocked(App.GetInstances).mockResolvedValue({
      root: '',
      dir: '',
      present: {},
      packUrl: {},
      sizeBytes: {},
    })
    vi.mocked(App.GetPackStates).mockResolvedValue([])
    vi.mocked(App.GetGameStates).mockResolvedValue([])
  })
  afterEach(() => {
    cleanup()
    Reflect.deleteProperty(window, 'go')
  })

  it('lists the situations Go sends, the chapter ones apart from Prism', async () => {
    render(<PreviewSection onStarted={() => undefined} />)
    const chapterGroup = await screen.findByRole('group', { name: 'Chapter situations' })
    expect(chapterGroup).toHaveTextContent('Crashed, with a crash report')
    expect(chapterGroup).toHaveTextContent('Running, to try Stop')
    expect(chapterGroup).not.toHaveTextContent('Prism missing')
    expect(screen.getByRole('group', { name: 'Prism situations' })).toHaveTextContent(
      'Prism missing',
    )
    expect(screen.getByText(/writes nothing, starts nothing and reaches no network/)).toBeVisible()
  })

  it('previews for the open chapter and reports it started', async () => {
    const onStarted = vi.fn()
    render(<PreviewSection onStarted={onStarted} />)
    expect(screen.getByRole('radio', { name: 'Frangfurd' })).toBeChecked()
    fireEvent.click(await screen.findByRole('button', { name: 'Crashed, with a crash report' }))
    await waitFor(() => expect(onStarted).toHaveBeenCalledExactlyOnceWith('frangfurd'))
    expect(App.StartPreview).toHaveBeenCalledExactlyOnceWith('frangfurd', 'crashed')
  })

  it('previews for the chapter picked instead', async () => {
    const onStarted = vi.fn()
    render(<PreviewSection onStarted={onStarted} />)
    const other = BUNDLED_MANIFEST.chapters[1]!
    fireEvent.click(screen.getByRole('radio', { name: other.name }))
    expect(screen.getByRole('radio', { name: other.name })).toBeChecked()
    fireEvent.click(await screen.findByRole('button', { name: 'Running, to try Stop' }))
    await waitFor(() => expect(onStarted).toHaveBeenCalledWith(other.id))
    expect(App.StartPreview).toHaveBeenCalledWith(other.id, 'running')
  })

  it("shows Go's refusal and does not claim a preview started", async () => {
    vi.mocked(App.StartPreview).mockRejectedValue(
      'Frangfurd is starting or running; close the game first',
    )
    const onStarted = vi.fn()
    render(<PreviewSection onStarted={onStarted} />)
    fireEvent.click(await screen.findByRole('button', { name: 'Crashed, with a crash report' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('starting or running')
    expect(onStarted).not.toHaveBeenCalled()
  })

  it('clears every preview', async () => {
    render(<PreviewSection onStarted={() => undefined} />)
    fireEvent.click(screen.getByRole('button', { name: 'Clear previews' }))
    await waitFor(() => expect(App.ClearPreviews).toHaveBeenCalledOnce())
    expect(App.GetGameStates).toHaveBeenCalled()
  })

  it('says a card preview shows the bar only while the loading splash is off', () => {
    const { rerender } = render(<PreviewSection onStarted={() => undefined} />)
    expect(screen.queryByText(/loading splash is off here/)).toBeNull()
    useSettingsStore.setState({ settings: { ...DEFAULT_SETTINGS, loadingSplashOn: false } })
    rerender(<PreviewSection onStarted={() => undefined} />)
    expect(screen.getByText(/loading splash is off here/)).toBeInTheDocument()
  })
})
