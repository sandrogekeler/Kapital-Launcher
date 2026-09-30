import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import * as App from '../../../wailsjs/go/main/App'
import { models } from '../../../wailsjs/go/models'
import { useEngineStore } from '../../stores/useEngineStore'
import { BUNDLED_MANIFEST } from '../../lib/manifest'
import type { ChapterSettingsInfo } from '../../types'
import { ChapterSettingsPanel } from './ChapterSettingsPanel'

vi.mock('../../../wailsjs/go/main/App')

const frangfurd = BUNDLED_MANIFEST.chapters[2]!
const info = (over: Partial<ChapterSettingsInfo> = {}) =>
  models.ChapterSettingsInfo.createFrom({
    chapterId: 'frangfurd',
    settings: { maxMemoryMb: 8192, jvm: 'zgc' },
    machineMemoryMb: 32768,
    prismDefaultMb: 4096,
    packMemoryMb: 8192,
    presets: ['zgc'],
    running: false,
    ...over,
  })
const report = (present: boolean) =>
  models.InstanceReport.createFrom({
    root: 'C:/Prism',
    dir: 'C:/Prism/instances',
    present: { frangfurd: present },
    packUrl: {},
    sizeBytes: {},
  })

describe('ChapterSettingsPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.assign(window, { go: {} })
    useEngineStore.setState({ instances: report(true), chapterSettings: {} })
    vi.mocked(App.GetChapterSettings).mockResolvedValue(info())
  })
  afterEach(() => {
    cleanup()
    Reflect.deleteProperty(window, 'go')
  })

  it('reads the instance and shows memory, the marks and the preset', async () => {
    render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
    expect(await screen.findByLabelText('Memory')).toHaveValue('8192')
    expect(screen.getByText('8.0 GB')).toBeInTheDocument()
    expect(
      screen.getByText(/Prism would pick 4\.0 GB; the pack recommends 8\.0 GB/),
    ).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: /ZGC/ })).toBeChecked()
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  })

  it('saves a changed memory and preset together and shows what Go wrote', async () => {
    vi.mocked(App.SaveChapterSettings).mockResolvedValue(
      info({ settings: { maxMemoryMb: 6144, jvm: '' } }),
    )
    render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
    const slider = await screen.findByLabelText('Memory')
    fireEvent.change(slider, { target: { value: '6144' } })
    fireEvent.click(screen.getByRole('radio', { name: /Prism's own/ }))
    expect(screen.getByText('6.0 GB')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(App.SaveChapterSettings).toHaveBeenCalledWith('frangfurd', {
        maxMemoryMb: 6144,
        jvm: '',
      }),
    )
    await waitFor(() => expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled())
    expect(screen.getByRole('radio', { name: /Prism's own/ })).toBeChecked()
  })

  it('shows a rejection and keeps the draft', async () => {
    vi.mocked(App.SaveChapterSettings).mockRejectedValue(
      'Frangfurd looks to be running; close the game first',
    )
    render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
    fireEvent.change(await screen.findByLabelText('Memory'), { target: { value: '4096' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await screen.findByText(/looks to be running/)
    expect(screen.getByLabelText('Memory')).toHaveValue('4096')
  })

  it('refuses to save while the instance runs, and says so', async () => {
    vi.mocked(App.GetChapterSettings).mockResolvedValue(info({ running: true }))
    render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
    fireEvent.change(await screen.findByLabelText('Memory'), { target: { value: '4096' } })
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
    expect(screen.getByText(/Close the game/)).toBeInTheDocument()
  })

  it('explains when the chapter is not installed', () => {
    useEngineStore.setState({ instances: report(false) })
    render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
    expect(screen.getByText(/not installed yet/)).toBeInTheDocument()
    expect(App.GetChapterSettings).not.toHaveBeenCalled()
  })

  it('closes on Back and Escape', async () => {
    const onClose = vi.fn()
    render(<ChapterSettingsPanel chapter={frangfurd} onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: 'Back' }))
    await act(async () => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    })
    expect(onClose).toHaveBeenCalledTimes(2)
  })
})
