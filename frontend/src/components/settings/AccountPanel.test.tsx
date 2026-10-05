import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import * as App from '../../../wailsjs/go/main/App'
import { BUNDLED_MANIFEST } from '../../lib/manifest'
import { useEngineStore } from '../../stores/useEngineStore'
import { DEFAULT_SETTINGS, useSettingsStore } from '../../stores/useSettingsStore'
import { AccountPanel } from './AccountPanel'

vi.mock('../../../wailsjs/go/main/App')

describe('AccountPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // A real backend answers: rejections revert and show (.claude/rules/ipc.md).
    Object.assign(window, { go: {} })
    vi.mocked(App.SaveSettings).mockResolvedValue()
    vi.mocked(App.GetPlayTime).mockResolvedValue([])
    useEngineStore.setState({ playTimes: {} })
    useSettingsStore.setState({
      settings: { ...DEFAULT_SETTINGS, profileName: 'Snadrochka' },
      loaded: true,
      error: null,
    })
  })
  afterEach(() => {
    cleanup()
    Reflect.deleteProperty(window, 'go')
  })

  it('shows the profile name and saves a new one on Enter', async () => {
    render(<AccountPanel onClose={() => undefined} />)
    expect(screen.getByText('Snadrochka', { selector: 'span' })).toBeInTheDocument()
    expect(screen.getByText('S', { selector: 'span' })).toBeInTheDocument()
    const field = screen.getByLabelText('Profile name')
    expect(field).toHaveValue('Snadrochka')
    fireEvent.change(field, { target: { value: ' Notch ' } })
    fireEvent.keyDown(field, { key: 'Enter' })
    await waitFor(() =>
      expect(App.SaveSettings).toHaveBeenCalledWith(
        expect.objectContaining({ profileName: 'Notch' }),
      ),
    )
    expect(await screen.findByText('Notch', { selector: 'span' })).toBeInTheDocument()
  })

  it('warns under a name Minecraft would not take, and says nothing for an empty one', () => {
    useSettingsStore.setState({ settings: { ...DEFAULT_SETTINGS, profileName: 'a b' } })
    render(<AccountPanel onClose={() => undefined} />)
    expect(screen.getByText(/3 to 16 letters, digits or _/)).toBeInTheDocument()
    cleanup()
    useSettingsStore.setState({ settings: { ...DEFAULT_SETTINGS, profileName: '' } })
    render(<AccountPanel onClose={() => undefined} />)
    expect(screen.queryByText(/3 to 16 letters/)).toBeNull()
    expect(screen.getByText("Prism's default account", { selector: 'span' })).toBeInTheDocument()
  })

  it('shows a refusal under the field and puts the old name back', async () => {
    vi.mocked(App.SaveSettings).mockRejectedValueOnce(
      'profile name "-x" could be read as an option',
    )
    render(<AccountPanel onClose={() => undefined} />)
    const field = screen.getByLabelText('Profile name')
    fireEvent.change(field, { target: { value: '-x' } })
    fireEvent.keyDown(field, { key: 'Enter' })
    await screen.findByText(/could be read as an option/)
    expect(field).toHaveValue('Snadrochka')
  })

  it('opens Prism, and says so when it cannot', async () => {
    vi.mocked(App.OpenPrism).mockResolvedValueOnce(undefined)
    render(<AccountPanel onClose={() => undefined} />)
    fireEvent.click(screen.getByRole('button', { name: 'Open Prism' }))
    await waitFor(() => expect(App.OpenPrism).toHaveBeenCalledOnce())
    vi.mocked(App.OpenPrism).mockRejectedValueOnce('could not open Prism: not found')
    fireEvent.click(screen.getByRole('button', { name: 'Open Prism' }))
    expect(await screen.findByText('could not open Prism: not found')).toBeInTheDocument()
    expect(screen.getByText(/never sees the account/)).toBeInTheDocument()
  })

  it('turns offline on with the profile name, and shows the name field only then', async () => {
    render(<AccountPanel onClose={() => undefined} />)
    expect(screen.queryByLabelText('Offline name')).toBeNull()
    expect(screen.queryByText('Online servers refuse an offline player.')).toBeNull()
    expect(screen.getByText('Microsoft account · via Prism')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('switch', { name: 'Play offline' }))
    await waitFor(() =>
      expect(App.SaveSettings).toHaveBeenCalledWith(
        expect.objectContaining({ offline: true, offlineName: 'Snadrochka' }),
      ),
    )
    expect(await screen.findByLabelText('Offline name')).toHaveValue('Snadrochka')
    expect(screen.getByText('Online servers refuse an offline player.')).toBeInTheDocument()
    expect(screen.getByText('Offline', { selector: 'span' })).toBeInTheDocument()
    expect(screen.queryByText('Microsoft account · via Prism')).toBeNull()
  })

  it('saves an offline name on Enter and turns offline off again', async () => {
    useSettingsStore.setState({
      settings: {
        ...DEFAULT_SETTINGS,
        profileName: 'Snadrochka',
        offline: true,
        offlineName: 'Steve',
      },
    })
    render(<AccountPanel onClose={() => undefined} />)
    const field = screen.getByLabelText('Offline name')
    fireEvent.change(field, { target: { value: ' Alex_01 ' } })
    fireEvent.keyDown(field, { key: 'Enter' })
    await waitFor(() =>
      expect(App.SaveSettings).toHaveBeenCalledWith(
        expect.objectContaining({ offline: true, offlineName: 'Alex_01' }),
      ),
    )
    fireEvent.click(screen.getByRole('switch', { name: 'Play offline' }))
    await waitFor(() =>
      expect(App.SaveSettings).toHaveBeenLastCalledWith(
        expect.objectContaining({ offline: false }),
      ),
    )
    expect(screen.queryByLabelText('Offline name')).toBeNull()
  })

  it('opens the name field first when there is no name to start from', async () => {
    useSettingsStore.setState({ settings: { ...DEFAULT_SETTINGS, profileName: '' } })
    render(<AccountPanel onClose={() => undefined} />)
    fireEvent.click(screen.getByRole('switch', { name: 'Play offline' }))
    const field = await screen.findByLabelText('Offline name')
    expect(App.SaveSettings).not.toHaveBeenCalled()
    fireEvent.change(field, { target: { value: 'Steve' } })
    fireEvent.keyDown(field, { key: 'Enter' })
    await waitFor(() =>
      expect(App.SaveSettings).toHaveBeenCalledWith(
        expect.objectContaining({ offline: true, offlineName: 'Steve' }),
      ),
    )
  })

  it("shows Go's refusal under the switch and puts it back off", async () => {
    vi.mocked(App.SaveSettings).mockRejectedValueOnce(
      'settings: offline name "x" is not a Minecraft name',
    )
    useSettingsStore.setState({
      settings: { ...DEFAULT_SETTINGS, profileName: 'Snadrochka', offlineName: 'x' },
    })
    render(<AccountPanel onClose={() => undefined} />)
    fireEvent.click(screen.getByRole('switch', { name: 'Play offline' }))
    expect(await screen.findByText(/is not a Minecraft name/)).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: 'Play offline' })).not.toBeChecked()
  })

  it('lists each chapter with its time, share and last play, and the total', async () => {
    const [first, second, third] = BUNDLED_MANIFEST.chapters
    const now = Date.now()
    vi.mocked(App.GetPlayTime).mockResolvedValue([
      { chapterId: first!.id, totalSeconds: 3 * 3600, lastLaunchMs: now },
      {
        chapterId: second!.id,
        totalSeconds: 1 * 3600 + 20 * 60,
        lastLaunchMs: now - 3 * 86_400_000,
      },
    ])
    render(<AccountPanel onClose={() => undefined} />)
    await waitFor(() => expect(App.GetPlayTime).toHaveBeenCalledOnce())
    expect(await screen.findByText('4 h 20 min')).toBeInTheDocument()
    expect(screen.getByText('3 h')).toBeInTheDocument()
    expect(screen.getByText('1 h 20 min')).toBeInTheDocument()
    expect(screen.getByText('Last played today')).toBeInTheDocument()
    expect(screen.getByText('Last played 3 days ago')).toBeInTheDocument()
    expect(screen.getByText('Not installed')).toBeInTheDocument()
    const row = screen.getByText(first!.name).closest('[data-chapter]')
    expect(row).toHaveAttribute('data-chapter', first!.id)
    expect(screen.getByText(third!.name).closest('[data-chapter]')).toHaveAttribute(
      'data-chapter',
      third!.id,
    )
    // The bar's share is a custom property, 3 of 4 h 20 min.
    expect(
      row
        ?.querySelector<HTMLElement>('[class*="w-(--play-share)"]')
        ?.style.getPropertyValue('--play-share'),
    ).toBe('69%')
  })

  it('reads the play time again when the window regains focus', async () => {
    render(<AccountPanel onClose={() => undefined} />)
    await waitFor(() => expect(App.GetPlayTime).toHaveBeenCalledTimes(1))
    await act(async () => {
      window.dispatchEvent(new Event('focus'))
    })
    await waitFor(() => expect(App.GetPlayTime).toHaveBeenCalledTimes(2))
  })

  it('closes on Back and on Escape', async () => {
    const onClose = vi.fn()
    render(<AccountPanel onClose={onClose} />)
    await act(async () => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    })
    fireEvent.click(screen.getByRole('button', { name: 'Back' }))
    expect(onClose).toHaveBeenCalledTimes(2)
  })
})
