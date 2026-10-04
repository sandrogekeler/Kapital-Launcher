import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import * as App from '../../../wailsjs/go/main/App'
import { models } from '../../../wailsjs/go/models'
import { useEngineStore } from '../../stores/useEngineStore'
import { useGameStore } from '../../stores/useGameStore'
import { DEFAULT_SETTINGS, useSettingsStore } from '../../stores/useSettingsStore'
import { BUNDLED_MANIFEST } from '../../lib/manifest'
import type { ChapterSettingsInfo, GamePhase } from '../../types'
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
const report = (present: boolean, packUrl: Record<string, string> = {}) =>
  models.InstanceReport.createFrom({
    root: 'C:/Prism',
    dir: 'C:/Prism/instances',
    present: { frangfurd: present },
    packUrl,
    sizeBytes: {},
  })
const phaseOf = (phase: GamePhase) => ({
  frangfurd: {
    chapterId: 'frangfurd',
    phase,
    since: '2026-10-02T10:00:00Z',
    startedAt: '2026-10-02T09:59:00Z',
  },
})
const published = frangfurd.pack.packwiz!
const local = 'http://localhost:8080/pack.toml'
const withOverride = (url: string | undefined) =>
  useSettingsStore.setState({
    settings: { ...DEFAULT_SETTINGS, packOverrides: url ? { frangfurd: url } : undefined },
  })

describe('ChapterSettingsPanel', () => {
  // The pack source section is its own chunk; loading it first keeps the tests off the transform.
  beforeAll(async () => {
    await import('./PackSourceSection')
    await import('./ModsSection')
  }, 60_000)

  beforeEach(() => {
    vi.clearAllMocks()
    Object.assign(window, { go: {} })
    useEngineStore.setState({
      instances: report(true),
      chapterSettings: {},
      installing: null,
    })
    useSettingsStore.setState({ settings: DEFAULT_SETTINGS })
    useGameStore.setState({ states: {} })
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
      "Frangfurd's game log changed less than a minute ago: if the game is closed, try again in a moment",
    )
    render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
    fireEvent.change(await screen.findByLabelText('Memory'), { target: { value: '4096' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await screen.findByText(/game log changed less than a minute ago/)
    expect(screen.getByLabelText('Memory')).toHaveValue('4096')
  })

  it('only hints when the log says running and the tracker is idle, and Save stays enabled', async () => {
    vi.mocked(App.GetChapterSettings).mockResolvedValue(info({ running: true }))
    render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
    fireEvent.change(await screen.findByLabelText('Memory'), { target: { value: '4096' } })
    expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
    expect(
      screen.getByText('Frangfurd may be running: its game log changed in the last minute.'),
    ).toBeInTheDocument()
    expect(screen.queryByText(/Close the game/)).toBeNull()
  })

  it('disables Save and warns while the tracker says the game is active', async () => {
    useGameStore.setState({ states: phaseOf('running') })
    vi.mocked(App.GetChapterSettings).mockResolvedValue(info({ running: true }))
    render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
    fireEvent.change(await screen.findByLabelText('Memory'), { target: { value: '4096' } })
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
    expect(screen.getByText(/Close the game/)).toBeInTheDocument()
    expect(screen.queryByText(/may be running/)).toBeNull()
  })

  it('says nothing about running when neither answer does', async () => {
    render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
    await screen.findByLabelText('Memory')
    expect(screen.queryByText(/running/)).toBeNull()
  })

  it('reads the info again once when the chapter run ends, not before and not for others', async () => {
    useGameStore.setState({ states: phaseOf('running') })
    render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
    await screen.findByLabelText('Memory')
    expect(App.GetChapterSettings).toHaveBeenCalledTimes(1)

    act(() => useGameStore.setState({ states: phaseOf('stopping') }))
    expect(App.GetChapterSettings).toHaveBeenCalledTimes(1)

    act(() => useGameStore.setState({ states: phaseOf('closed') }))
    await waitFor(() => expect(App.GetChapterSettings).toHaveBeenCalledTimes(2))
    act(() => useGameStore.setState({ states: phaseOf('idle') }))
    expect(App.GetChapterSettings).toHaveBeenCalledTimes(2)
  })

  it('reads the info again when the window regains focus, and stops listening on close', async () => {
    const { unmount } = render(
      <ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />,
    )
    await screen.findByLabelText('Memory')
    expect(App.GetChapterSettings).toHaveBeenCalledTimes(1)
    await act(async () => {
      window.dispatchEvent(new Event('focus'))
    })
    expect(App.GetChapterSettings).toHaveBeenCalledTimes(2)
    unmount()
    window.dispatchEvent(new Event('focus'))
    expect(App.GetChapterSettings).toHaveBeenCalledTimes(2)
  })

  it('shows the mods section for an installed chapter, after the memory and before the pack source', async () => {
    vi.mocked(App.GetChapterMods).mockResolvedValue({
      chapterId: 'frangfurd',
      running: false,
      mods: [
        { name: 'DistantHorizons-3.3.3-1.21.1-fabric-neoforge.jar', disabled: false, size: 1 },
      ],
      toggles: [
        {
          name: 'Distant Horizons',
          jarPrefix: 'DistantHorizons-',
          jars: ['DistantHorizons-3.3.3-1.21.1-fabric-neoforge.jar'],
          disabled: false,
        },
      ],
    } as never)
    render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
    expect(await screen.findByRole('switch', { name: 'Distant Horizons' })).toBeChecked()
    expect(App.GetChapterMods).toHaveBeenCalledWith('frangfurd')
    const headings = screen.getAllByRole('heading', { level: 3 }).map((h) => h.textContent)
    expect(headings.indexOf('Mods')).toBeGreaterThanOrEqual(0)
  })

  it('does not read the mods of a chapter that is not installed', () => {
    useEngineStore.setState({ instances: report(false) })
    render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
    expect(App.GetChapterMods).not.toHaveBeenCalled()
    expect(screen.queryByRole('region', { name: 'Mods' })).toBeNull()
  })

  it('explains when the chapter is not installed', () => {
    useEngineStore.setState({ instances: report(false) })
    render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
    expect(screen.getByText(/not installed yet/)).toBeInTheDocument()
    expect(App.GetChapterSettings).not.toHaveBeenCalled()
  })

  it('shows the Server section for a chapter that is not installed, and none for a chapter with no server', () => {
    useEngineStore.setState({ instances: report(false) })
    const { unmount } = render(
      <ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />,
    )
    expect(screen.getByRole('heading', { name: 'Server' })).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: 'Join the server on Play' })).toBeInTheDocument()
    unmount()
    render(
      <ChapterSettingsPanel chapter={{ ...frangfurd, server: null }} onClose={() => undefined} />,
    )
    expect(screen.queryByRole('heading', { name: 'Server' })).toBeNull()
  })

  it('puts the Server section after the memory form and before the mods (issue 163)', async () => {
    vi.mocked(App.GetChapterMods).mockResolvedValue({
      chapterId: 'frangfurd',
      running: false,
      mods: [],
      toggles: [],
    } as never)
    render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
    const memory = await screen.findByLabelText('Memory')
    const server = screen.getByRole('heading', { name: 'Server' })
    const mods = await screen.findByRole('heading', { name: 'Mods' })
    expect(memory.compareDocumentPosition(server) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(server.compareDocumentPosition(mods) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  describe('reveal', () => {
    it('holds the form back, with a faint line, until the instance has been read, then reveals it', async () => {
      let finish: (v: ChapterSettingsInfo) => void = () => undefined
      vi.mocked(App.GetChapterSettings).mockReturnValue(new Promise((r) => (finish = r)) as never)
      render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
      expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(/^Settings$/)
      expect(screen.getByText('Reading.')).toHaveClass('reveal-wait')
      expect(screen.queryByLabelText('Memory')).toBeNull()
      expect(document.querySelector('.reveal')).toBeNull()
      await act(async () => finish(info()))
      expect(screen.getByLabelText('Memory').closest('.reveal')).not.toBeNull()
      expect(screen.queryByText('Reading.')).toBeNull()
    })

    it('is ready at once for an instance that is not there and for one that cannot be looked for', () => {
      useEngineStore.setState({ instances: report(false) })
      const { unmount } = render(
        <ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />,
      )
      expect(screen.queryByText('Reading.')).toBeNull()
      expect(screen.getByText(/not installed yet/).closest('.reveal')).not.toBeNull()
      unmount()
      useEngineStore.setState({ instances: null })
      render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
      expect(screen.queryByText('Reading.')).toBeNull()
      expect(screen.getByText(/could not be looked for/).closest('.reveal')).not.toBeNull()
    })
  })

  it('opens the instance folder by chapter id and shows a failure', async () => {
    vi.mocked(App.OpenInstanceFolder).mockResolvedValueOnce(undefined)
    render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
    await screen.findByLabelText('Memory')
    fireEvent.click(screen.getByRole('button', { name: 'Open folder' }))
    await waitFor(() => expect(App.OpenInstanceFolder).toHaveBeenCalledWith('frangfurd'))
    expect(screen.queryByText(/could not open/)).not.toBeInTheDocument()

    vi.mocked(App.OpenInstanceFolder).mockRejectedValueOnce(
      'could not open the Frangfurd folder: no file manager',
    )
    fireEvent.click(screen.getByRole('button', { name: 'Open folder' }))
    await screen.findByText(/could not open the Frangfurd folder/)
  })

  it('keeps Open folder disabled until the instance exists', () => {
    useEngineStore.setState({ instances: report(false) })
    render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
    expect(screen.getByRole('button', { name: 'Open folder' })).toBeDisabled()
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

  describe('pack source', () => {
    const section = () => screen.queryByRole('region', { name: 'Pack source' })
    // The section is its own chunk, loaded when the panel asks; a test that
    // looks for its absence waits until the load would have landed.
    const settled = () => act(async () => await import('./PackSourceSection'))
    const shown = () => screen.findByRole('region', { name: 'Pack source' })
    const card = (name: string) => screen.getByRole('radio', { name })

    it('is not shown for a chapter that never left its published pack', async () => {
      useEngineStore.setState({ instances: report(true, { frangfurd: published }) })
      render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
      await settled()
      expect(section()).not.toBeInTheDocument()
    })

    it('is not shown while the chapter is not installed, override or not', async () => {
      withOverride(local)
      useEngineStore.setState({ instances: report(false) })
      render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
      await settled()
      expect(section()).not.toBeInTheDocument()
    })

    it('is shown when settings hold a local pack, the published one marked current', async () => {
      withOverride(local)
      useEngineStore.setState({ instances: report(true, { frangfurd: published }) })
      render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
      await shown()
      expect(screen.getByText(new URL(published).host)).toBeInTheDocument()
      expect(screen.getByText('localhost:8080')).toBeInTheDocument()
      expect(screen.getAllByText('● Current')).toHaveLength(1)
      expect(card('Published pack')).toBeChecked()
      expect(card('Dev pack')).not.toBeChecked()
      expect(card('Dev pack')).toBeEnabled()
      expect(screen.getByText('Switch')).toBeInTheDocument()
      expect(
        screen.getByText('The next Play syncs from the chosen pack. Saves and settings stay.'),
      ).toBeInTheDocument()
    })

    it('is shown for an instance that syncs from a local pack the setting no longer names', async () => {
      useEngineStore.setState({ instances: report(true, { frangfurd: local }) })
      render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
      await shown()
      expect(screen.getByText('localhost:8080')).toBeInTheDocument()
      expect(card('Dev pack')).toBeChecked()
      expect(card('Published pack')).toBeEnabled()
      expect(screen.getAllByText('● Current')).toHaveLength(1)
    })

    it('switches on click through the store and shows the other pack as current', async () => {
      withOverride(local)
      useEngineStore.setState({ instances: report(true, { frangfurd: published }) })
      vi.mocked(App.SetPackSource).mockResolvedValue(report(true, { frangfurd: local }))
      vi.mocked(App.GetPackStates).mockResolvedValue([])
      render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
      await shown()
      fireEvent.click(card('Dev pack'))
      await waitFor(() => expect(App.SetPackSource).toHaveBeenCalledWith('frangfurd', 'dev'))
      await waitFor(() => expect(card('Dev pack')).toBeChecked())
      expect(card('Published pack')).toBeEnabled()
      expect(screen.getAllByText('● Current')).toHaveLength(1)
    })

    it('shows a refusal in the section and keeps the choice as it was', async () => {
      withOverride(local)
      useEngineStore.setState({ instances: report(true, { frangfurd: published }) })
      vi.mocked(App.SetPackSource).mockRejectedValue(
        "Frangfurd: pack source: the launcher did not write this instance's pre-launch command, so it will not change it",
      )
      render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
      await shown()
      fireEvent.click(card('Dev pack'))
      await screen.findByText(/did not write/)
      expect(card('Dev pack')).toBeEnabled()
      expect(card('Published pack')).toBeChecked()
    })

    it('waits while the chapter is installing or its game is active', async () => {
      withOverride(local)
      useEngineStore.setState({
        instances: report(true, { frangfurd: published }),
        installing: 'frangfurd',
      })
      const { unmount } = render(
        <ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />,
      )
      await shown()
      expect(card('Dev pack')).toBeDisabled()
      unmount()

      useEngineStore.setState({ installing: null })
      useGameStore.setState({ states: phaseOf('running') })
      vi.mocked(App.GetChapterSettings).mockResolvedValue(info({ running: true }))
      render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
      await shown()
      expect(card('Dev pack')).toBeDisabled()
      expect(screen.getByText('Close the game to switch.')).toBeInTheDocument()
      expect(screen.queryByText(/may be running/)).toBeNull()
    })

    it('only hints, and keeps the switch enabled, when just the log says running', async () => {
      withOverride(local)
      useEngineStore.setState({ instances: report(true, { frangfurd: published }) })
      vi.mocked(App.GetChapterSettings).mockResolvedValue(info({ running: true }))
      render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
      await shown()
      await waitFor(() =>
        expect(useEngineStore.getState().chapterSettings.frangfurd?.running).toBe(true),
      )
      expect(card('Dev pack')).toBeEnabled()
      const region = within(screen.getByRole('region', { name: 'Pack source' }))
      expect(region.getByText(/Frangfurd may be running/)).toBeInTheDocument()
      expect(screen.queryByText('Close the game to switch.')).toBeNull()
    })

    it('disables a pack that is not there, with the reason', async () => {
      const unpublished = { ...frangfurd, pack: { ...frangfurd.pack, packwiz: null } }
      withOverride(local)
      useEngineStore.setState({ instances: report(true, { frangfurd: local }) })
      render(<ChapterSettingsPanel chapter={unpublished} onClose={() => undefined} />)
      await shown()
      expect(card('Published pack')).toBeDisabled()
      expect(screen.getByText('No published pack yet.')).toBeInTheDocument()
    })

    it('disables both for an instance whose command the launcher did not write', async () => {
      withOverride(local)
      render(<ChapterSettingsPanel chapter={frangfurd} onClose={() => undefined} />)
      await shown()
      expect(card('Dev pack')).toBeDisabled()
      expect(card('Published pack')).toBeDisabled()
      expect(screen.getAllByText('Not made by this launcher.')).toHaveLength(2)
    })
  })
})
