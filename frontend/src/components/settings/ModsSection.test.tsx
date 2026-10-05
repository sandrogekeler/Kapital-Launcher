import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import * as App from '../../../wailsjs/go/main/App'
import { useGameStore } from '../../stores/useGameStore'
import { useModStore } from '../../stores/useModStore'
import { BUNDLED_MANIFEST } from '../../lib/manifest'
import type { ChapterMods, GamePhase } from '../../types'
import { ModsSection } from './ModsSection'

vi.mock('../../../wailsjs/go/main/App')

const frangfurd = BUNDLED_MANIFEST.chapters[2]!
const dh = 'DistantHorizons-3.3.3-1.21.1-fabric-neoforge.jar'
const cw = 'colorwheel-neoforge-1.3.0+mc1.21.1.jar'
const jade = 'Jade-1.21.1-NeoForge-15.1.jar'

/** The view Go answers with: Distant Horizons and Colorwheel as listed, Jade a plain mod. */
const view = (off: string[] = [], running = false): ChapterMods => ({
  chapterId: 'frangfurd',
  running,
  mods: [
    { name: cw, disabled: off.includes(cw), size: 120_000 },
    { name: dh, disabled: off.includes(dh), size: 2_400_000 },
    { name: jade, disabled: off.includes(jade), size: 90_000 },
  ],
  toggles: [
    {
      name: 'Distant Horizons',
      jarPrefix: 'DistantHorizons-',
      jars: [dh],
      disabled: off.includes(dh),
      blocked: false,
    },
    {
      name: 'Colorwheel',
      jarPrefix: 'colorwheel-neoforge-',
      jars: [cw],
      disabled: off.includes(cw),
      blocked: false,
    },
    {
      name: 'Create Better FPS',
      jarPrefix: 'createbetterfps-',
      jars: [],
      disabled: false,
      blocked: false,
    },
  ],
})
const phaseOf = (phase: GamePhase) => ({
  frangfurd: {
    chapterId: 'frangfurd',
    phase,
    since: '2026-10-02T10:00:00Z',
    startedAt: '2026-10-02T09:59:00Z',
  },
})

describe('ModsSection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.assign(window, { go: {} })
    useModStore.setState({ byChapter: {}, unavailable: false, readError: {}, writing: null })
    useGameStore.setState({ states: {} })
    vi.mocked(App.GetChapterMods).mockResolvedValue(view([dh]) as never)
  })
  afterEach(() => {
    cleanup()
    Reflect.deleteProperty(window, 'go')
  })

  it('shows the manifest’s quick switches as the mods are, on or off', async () => {
    render(<ModsSection chapter={frangfurd} />)
    expect(await screen.findByRole('switch', { name: 'Distant Horizons' })).not.toBeChecked()
    expect(screen.getByRole('switch', { name: 'Colorwheel' })).toBeChecked()
    // No file name under a switch that can be used.
    expect(
      screen.getByRole('switch', { name: 'Distant Horizons' }),
    ).not.toHaveAccessibleDescription()
    // A mod the pack has not downloaded yet cannot be switched, and says why.
    const better = screen.getByRole('switch', { name: 'Create Better FPS' })
    expect(better).toBeDisabled()
    expect(better).toHaveAccessibleDescription(/Not downloaded yet/)
    expect(
      screen.getByText('Changes apply on the next Play. A disabled mod is not downloaded again.'),
    ).toBeInTheDocument()
    expect(App.GetChapterMods).toHaveBeenCalledWith('frangfurd')
  })

  it('greys out and locks a mod whose requirement is off, and says which (Colorwheel needs Iris)', async () => {
    const base = view([cw])
    vi.mocked(App.GetChapterMods).mockResolvedValue({
      ...base,
      toggles: [
        {
          name: 'Iris',
          jarPrefix: 'iris-neoforge-',
          jars: ['iris-neoforge-1.8.14.jar'],
          disabled: true,
          blocked: false,
        },
        { ...base.toggles[1]!, requires: 'iris-neoforge-', blocked: true },
      ],
    } as never)
    render(<ModsSection chapter={frangfurd} />)
    const colorwheel = await screen.findByRole('switch', { name: /Colorwheel/ })
    expect(colorwheel).toBeDisabled()
    expect(colorwheel).not.toBeChecked()
    expect(screen.getByText('Needs Iris')).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: 'Iris' })).toBeEnabled()
  })

  it('switches a mod off with the whole list Go keeps, and shows what Go answers', async () => {
    vi.mocked(App.SetModsDisabled).mockResolvedValue(view([dh, cw]) as never)
    render(<ModsSection chapter={frangfurd} />)
    fireEvent.click(await screen.findByRole('switch', { name: 'Colorwheel' }))
    await waitFor(() =>
      expect(App.SetModsDisabled).toHaveBeenCalledWith(
        'frangfurd',
        expect.arrayContaining([dh, cw]),
      ),
    )
    expect(vi.mocked(App.SetModsDisabled).mock.calls[0]?.[1]).toHaveLength(2)
    await waitFor(() =>
      expect(screen.getByRole('switch', { name: 'Colorwheel' })).not.toBeChecked(),
    )
  })

  it('switches a mod back on', async () => {
    vi.mocked(App.SetModsDisabled).mockResolvedValue(view() as never)
    render(<ModsSection chapter={frangfurd} />)
    fireEvent.click(await screen.findByRole('switch', { name: 'Distant Horizons' }))
    await waitFor(() => expect(App.SetModsDisabled).toHaveBeenCalledWith('frangfurd', []))
    await waitFor(() =>
      expect(screen.getByRole('switch', { name: 'Distant Horizons' })).toBeChecked(),
    )
  })

  it('shows a refusal in the error style and puts the switch back', async () => {
    vi.mocked(App.SetModsDisabled).mockRejectedValue(
      "Frangfurd's game log changed less than a minute ago: if the game is closed, try again in a moment",
    )
    render(<ModsSection chapter={frangfurd} />)
    fireEvent.click(await screen.findByRole('switch', { name: 'Colorwheel' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(
      /game log changed less than a minute ago/,
    )
    await waitFor(() => expect(screen.getByRole('switch', { name: 'Colorwheel' })).toBeChecked())
  })

  it('keeps the advanced list closed until asked, then lists every mod with a search', async () => {
    vi.mocked(App.SetModsDisabled).mockResolvedValue(view([dh, jade]) as never)
    render(<ModsSection chapter={frangfurd} />)
    const disclosure = await screen.findByRole('button', { name: 'Advanced mod control' })
    expect(disclosure).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByLabelText('Search mods')).toBeNull()

    fireEvent.click(disclosure)
    expect(disclosure).toHaveAttribute('aria-expanded', 'true')
    const list = screen.getByRole('list', { name: 'Mods' })
    const rows = within(list).getAllByRole('switch')
    expect(rows).toHaveLength(3)
    // Sizes are shown, names without the extension.
    expect(
      within(list).getByRole('switch', { name: 'Jade-1.21.1-NeoForge-15.1' }),
    ).toHaveAccessibleDescription('88 KB')
    expect(screen.getByText('3 mods, 1 switched off.')).toBeInTheDocument()

    // The search narrows the list by every word, in any case.
    fireEvent.change(screen.getByLabelText('Search mods'), { target: { value: 'jade neoforge' } })
    expect(within(list).getAllByRole('switch')).toHaveLength(1)
    expect(screen.getByText('1 of 3 mods.')).toBeInTheDocument()
    fireEvent.click(within(list).getByRole('switch', { name: 'Jade-1.21.1-NeoForge-15.1' }))
    await waitFor(() =>
      expect(App.SetModsDisabled).toHaveBeenCalledWith(
        'frangfurd',
        expect.arrayContaining([dh, jade]),
      ),
    )
    fireEvent.change(screen.getByLabelText('Search mods'), { target: { value: 'nothing like it' } })
    expect(screen.getByText('No mod has that name.')).toBeInTheDocument()

    fireEvent.click(disclosure)
    expect(screen.queryByLabelText('Search mods')).toBeNull()
  })

  it('says the mods appear after the first Play when the folder is empty', async () => {
    vi.mocked(App.GetChapterMods).mockResolvedValue({ ...view(), mods: [], toggles: [] } as never)
    render(<ModsSection chapter={frangfurd} />)
    fireEvent.click(await screen.findByRole('button', { name: 'Advanced mod control' }))
    expect(screen.getByText(/appear here after the first Play/)).toBeInTheDocument()
    expect(screen.queryByRole('switch')).toBeNull()
  })

  it('disables the switches while the game runs, and says why', async () => {
    useGameStore.setState({ states: phaseOf('running') })
    render(<ModsSection chapter={frangfurd} />)
    expect(await screen.findByRole('switch', { name: 'Colorwheel' })).toBeDisabled()
    expect(screen.getByRole('switch', { name: 'Distant Horizons' })).toBeDisabled()
    expect(screen.getByText('Close the game to change mods.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Advanced mod control' }))
    for (const row of within(screen.getByRole('list', { name: 'Mods' })).getAllByRole('switch')) {
      expect(row).toBeDisabled()
    }
  })

  it('only hints when the game log alone says it runs, and never disables a switch on it', async () => {
    vi.mocked(App.GetChapterMods).mockResolvedValue(view([], true) as never)
    render(<ModsSection chapter={frangfurd} />)
    expect(
      await screen.findByText(/may be running: its game log changed in the last minute/),
    ).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: 'Colorwheel' })).toBeEnabled()
    expect(screen.queryByText('Close the game to change mods.')).toBeNull()
  })

  it('reads again when the run ends and when the window comes back', async () => {
    useGameStore.setState({ states: phaseOf('running') })
    render(<ModsSection chapter={frangfurd} />)
    await screen.findByRole('switch', { name: 'Colorwheel' })
    expect(App.GetChapterMods).toHaveBeenCalledTimes(1)
    act(() => useGameStore.setState({ states: phaseOf('closed') }))
    await waitFor(() => expect(App.GetChapterMods).toHaveBeenCalledTimes(2))
    act(() => {
      window.dispatchEvent(new Event('focus'))
    })
    await waitFor(() => expect(App.GetChapterMods).toHaveBeenCalledTimes(3))
  })

  it('shows why the mods could not be read, and nothing at all with no bridge', async () => {
    vi.mocked(App.GetChapterMods).mockRejectedValue('Frangfurd is not installed')
    const { container } = render(<ModsSection chapter={frangfurd} />)
    expect(await screen.findByRole('alert')).toHaveTextContent('Frangfurd is not installed')
    cleanup()
    Reflect.deleteProperty(window, 'go')
    useModStore.setState({ byChapter: {}, readError: {}, unavailable: false })
    const bare = render(<ModsSection chapter={frangfurd} />)
    await waitFor(() => expect(useModStore.getState().unavailable).toBe(true))
    expect(bare.container).toBeEmptyDOMElement()
    expect(container).toBeDefined()
  })
})
