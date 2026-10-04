import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import * as App from '../../../wailsjs/go/main/App'
import { DEFAULT_SETTINGS, useSettingsStore } from '../../stores/useSettingsStore'
import { useChapterStore } from '../../stores/useChapterStore'
import { useEngineStore } from '../../stores/useEngineStore'
import type { EngineInfo } from '../../types'
import { SettingsPanel } from './SettingsPanel'

vi.mock('../../../wailsjs/go/main/App')

const engine: EngineInfo = {
  found: true,
  executable: 'C:\\Programs\\PrismLauncher\\prismlauncher.exe',
  version: '11.1.0',
  root: '',
  source: 'standard-location',
}

describe('SettingsPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // A real backend answers: rejections revert and show (.claude/rules/ipc.md).
    Object.assign(window, { go: {} })
    vi.mocked(App.SaveSettings).mockResolvedValue()
    vi.mocked(App.GetEngine).mockResolvedValue(engine)
    vi.mocked(App.GetInstances).mockResolvedValue({
      root: 'C:\\Users\\me\\AppData\\Roaming\\PrismLauncher',
      dir: '',
      present: {},
      packUrl: {},
      sizeBytes: {},
    })
    useSettingsStore.setState({ settings: DEFAULT_SETTINGS, loaded: true, error: null })
    useEngineStore.setState({ engine, instances: null })
  })
  afterEach(() => {
    cleanup()
    Reflect.deleteProperty(window, 'go')
  })

  it('shows what detection resolved in the empty fields', () => {
    render(<SettingsPanel onClose={() => undefined} onShowChapter={() => undefined} />)
    expect(screen.getByLabelText('Prism program')).toHaveAttribute('placeholder', engine.executable)
    expect(screen.getByText(/standard install location/)).toBeInTheDocument()
    expect(screen.getByLabelText('Prism data folder')).toHaveAttribute(
      'placeholder',
      "Prism's own data folder",
    )
  })

  it('saves the profile on Enter and the root on blur, then re-reads the engine', async () => {
    render(<SettingsPanel onClose={() => undefined} onShowChapter={() => undefined} />)
    const profile = screen.getByLabelText('Profile name')
    fireEvent.change(profile, { target: { value: ' Sandro ' } })
    fireEvent.keyDown(profile, { key: 'Enter' })
    await waitFor(() =>
      expect(App.SaveSettings).toHaveBeenCalledWith(
        expect.objectContaining({ profileName: 'Sandro' }),
      ),
    )
    expect(App.GetEngine).not.toHaveBeenCalled()

    const root = screen.getByLabelText('Prism data folder')
    fireEvent.change(root, { target: { value: 'D:\\prism' } })
    fireEvent.blur(root)
    await waitFor(() =>
      expect(App.SaveSettings).toHaveBeenLastCalledWith(
        expect.objectContaining({ prismRoot: 'D:\\prism', profileName: 'Sandro' }),
      ),
    )
    await waitFor(() => expect(App.GetEngine).toHaveBeenCalled())
    expect(App.GetInstances).toHaveBeenCalled()
  })

  it('does not save an unchanged field', () => {
    render(<SettingsPanel onClose={() => undefined} onShowChapter={() => undefined} />)
    const profile = screen.getByLabelText('Profile name')
    fireEvent.change(profile, { target: { value: '  ' } })
    fireEvent.blur(profile)
    fireEvent.keyDown(profile, { key: 'Enter' })
    expect(App.SaveSettings).not.toHaveBeenCalled()
  })

  it('shows a rejection under the field and puts the old value back', async () => {
    vi.mocked(App.SaveSettings).mockRejectedValueOnce(
      'settings: prism executable "D:\\\\nowhere.exe": file does not exist',
    )
    render(<SettingsPanel onClose={() => undefined} onShowChapter={() => undefined} />)
    const exe = screen.getByLabelText('Prism program')
    fireEvent.change(exe, { target: { value: 'D:\\nowhere.exe' } })
    fireEvent.keyDown(exe, { key: 'Enter' })
    await screen.findByText(/does not exist/)
    expect(exe).toHaveValue('')
    expect(exe).toHaveAttribute('aria-invalid', 'true')
    expect(useSettingsStore.getState().settings.prismExecutable).toBe('')

    // The next good save clears it.
    fireEvent.change(exe, { target: { value: 'D:\\prism\\prismlauncher.exe' } })
    fireEvent.keyDown(exe, { key: 'Enter' })
    await waitFor(() => expect(screen.queryByText(/does not exist/)).not.toBeInTheDocument())
  })

  it('commits a Browse pick and ignores a cancelled dialog', async () => {
    vi.mocked(App.ChoosePrismExecutable).mockResolvedValueOnce('')
    vi.mocked(App.ChoosePrismRoot).mockResolvedValueOnce('E:\\PrismData')
    render(<SettingsPanel onClose={() => undefined} onShowChapter={() => undefined} />)
    const [browseExe, browseRoot] = screen.getAllByRole('button', { name: 'Browse' })
    fireEvent.click(browseExe!)
    await waitFor(() => expect(App.ChoosePrismExecutable).toHaveBeenCalledOnce())
    expect(App.SaveSettings).not.toHaveBeenCalled()

    fireEvent.click(browseRoot!)
    await waitFor(() =>
      expect(App.SaveSettings).toHaveBeenCalledWith(
        expect.objectContaining({ prismRoot: 'E:\\PrismData' }),
      ),
    )
    expect(screen.getByLabelText('Prism data folder')).toHaveValue('E:\\PrismData')
  })

  it('shows a picker failure under its field', async () => {
    vi.mocked(App.ChoosePrismRoot).mockRejectedValueOnce('choose prism root: dialog failed')
    render(<SettingsPanel onClose={() => undefined} onShowChapter={() => undefined} />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Browse' })[1]!)
    await screen.findByText(/dialog failed/)
    expect(App.SaveSettings).not.toHaveBeenCalled()
  })

  it('holds its sections back until the settings are loaded, then reveals them', () => {
    useSettingsStore.setState({ loaded: false })
    render(<SettingsPanel onClose={() => undefined} onShowChapter={() => undefined} />)
    expect(screen.getByRole('heading', { level: 1, name: 'Settings' })).toBeInTheDocument()
    expect(screen.getByText('Reading.')).toHaveClass('reveal-wait')
    expect(screen.queryByLabelText('Prism program')).toBeNull()
    act(() => useSettingsStore.setState({ loaded: true }))
    expect(screen.queryByText('Reading.')).toBeNull()
    expect(screen.getByLabelText('Prism program').closest('.reveal')).not.toBeNull()
  })

  it('ends with the About section', () => {
    render(<SettingsPanel onClose={() => undefined} onShowChapter={() => undefined} />)
    expect(screen.getByRole('heading', { name: 'About' })).toBeInTheDocument()
  })

  it('has no Server section: the address and the join switch are on each chapter page (issue 163)', () => {
    render(<SettingsPanel onClose={() => undefined} onShowChapter={() => undefined} />)
    expect(screen.queryByRole('heading', { name: 'Server' })).toBeNull()
    expect(screen.queryByRole('radiogroup', { name: /Server for/ })).toBeNull()
    expect(screen.queryByRole('switch', { name: 'Join the server on Play' })).toBeNull()
  })

  it('saves the theme on click, and the arrow keys only move between the options', async () => {
    render(<SettingsPanel onClose={() => undefined} onShowChapter={() => undefined} />)
    expect(screen.getByRole('radio', { name: 'Dark' })).toBeChecked()
    fireEvent.click(screen.getByRole('radio', { name: 'Light' }))
    await waitFor(() =>
      expect(App.SaveSettings).toHaveBeenCalledWith(expect.objectContaining({ theme: 'light' })),
    )
    expect(screen.getByRole('radio', { name: 'Light' })).toBeChecked()
  })

  it('says where the map opens, in the launcher until chosen otherwise, and saves the choice (issue 161)', async () => {
    render(<SettingsPanel onClose={() => undefined} onShowChapter={() => undefined} />)
    expect(screen.getByRole('radiogroup', { name: 'Open the map' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'In the launcher' })).toBeChecked()
    expect(screen.getByRole('radio', { name: 'In the browser' })).not.toBeChecked()

    fireEvent.click(screen.getByRole('radio', { name: 'In the browser' }))
    await waitFor(() =>
      expect(App.SaveSettings).toHaveBeenCalledWith(expect.objectContaining({ mapIn: 'browser' })),
    )
    expect(screen.getByRole('radio', { name: 'In the browser' })).toBeChecked()

    fireEvent.click(screen.getByRole('radio', { name: 'In the launcher' }))
    await waitFor(() =>
      expect(App.SaveSettings).toHaveBeenLastCalledWith(expect.objectContaining({ mapIn: 'app' })),
    )
  })

  it('puts the map choice back and says why when Go refuses it', async () => {
    vi.mocked(App.SaveSettings).mockRejectedValue('settings: map location is not one of')
    render(<SettingsPanel onClose={() => undefined} onShowChapter={() => undefined} />)
    fireEvent.click(screen.getByRole('radio', { name: 'In the browser' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('map location')
    expect(screen.getByRole('radio', { name: 'In the launcher' })).toBeChecked()
  })

  it('moves the focus between theme options with the arrow keys and picks only on Enter or Space', () => {
    render(<SettingsPanel onClose={() => undefined} onShowChapter={() => undefined} />)
    const dark = screen.getByRole('radio', { name: 'Dark' })
    dark.focus()
    fireEvent.keyDown(dark, { key: 'ArrowRight' })
    expect(screen.getByRole('radio', { name: 'Light' })).toHaveFocus()
    fireEvent.keyDown(screen.getByRole('radio', { name: 'Light' }), { key: 'ArrowLeft' })
    fireEvent.keyDown(dark, { key: 'ArrowLeft' })
    expect(screen.getByRole('radio', { name: 'System' })).toHaveFocus()
    expect(App.SaveSettings).not.toHaveBeenCalled()
  })

  it('writes and clears a local pack address per chapter', async () => {
    render(<SettingsPanel onClose={() => undefined} onShowChapter={() => undefined} />)
    fireEvent.click(screen.getByRole('button', { name: 'Developer' }))
    const field = screen.getByLabelText('Local pack for Frangfurd')
    fireEvent.change(field, { target: { value: 'http://localhost:8080/pack.toml' } })
    fireEvent.keyDown(field, { key: 'Enter' })
    await waitFor(() =>
      expect(App.SaveSettings).toHaveBeenLastCalledWith(
        expect.objectContaining({
          packOverrides: { frangfurd: 'http://localhost:8080/pack.toml' },
        }),
      ),
    )
    fireEvent.change(field, { target: { value: '' } })
    fireEvent.blur(field)
    await waitFor(() => expect(App.SaveSettings).toHaveBeenCalledTimes(2))
    expect(vi.mocked(App.SaveSettings).mock.calls[1]?.[0].packOverrides).toBeUndefined()
  })

  it('closes on Back and on Escape, but Escape on a dirty field reverts it first', async () => {
    const onClose = vi.fn()
    render(<SettingsPanel onClose={onClose} onShowChapter={() => undefined} />)
    const profile = screen.getByLabelText('Profile name')
    fireEvent.change(profile, { target: { value: 'typo' } })
    fireEvent.keyDown(profile, { key: 'Escape' })
    expect(profile).toHaveValue('')
    expect(onClose).not.toHaveBeenCalled()

    await act(async () => {
      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    })
    expect(onClose).toHaveBeenCalledOnce()
    fireEvent.click(screen.getByRole('button', { name: 'Back' }))
    expect(onClose).toHaveBeenCalledTimes(2)
  })

  describe('loading splash toggle', () => {
    it('is hidden where Go says the splash is not available', () => {
      render(<SettingsPanel onClose={() => undefined} onShowChapter={() => undefined} />)
      expect(screen.queryByRole('switch', { name: 'Loading splash' })).not.toBeInTheDocument()
    })

    it('shows its state and hint, and saves the choice', async () => {
      useSettingsStore.setState({
        settings: { ...DEFAULT_SETTINGS, loadingSplashAvailable: true, loadingSplashOn: true },
        loaded: true,
        error: null,
      })
      render(<SettingsPanel onClose={() => undefined} onShowChapter={() => undefined} />)
      const box = screen.getByRole('switch', { name: 'Loading splash' })
      expect(box).toBeChecked()
      expect(
        screen.getByText(
          "Shows a small loading card from Play until the game's own loading screen.",
        ),
      ).toBeInTheDocument()

      fireEvent.click(box)
      await waitFor(() =>
        expect(App.SaveSettings).toHaveBeenLastCalledWith(
          expect.objectContaining({ loadingSplash: false }),
        ),
      )
      expect(box).not.toBeChecked()

      fireEvent.click(box)
      await waitFor(() =>
        expect(App.SaveSettings).toHaveBeenLastCalledWith(
          expect.objectContaining({ loadingSplash: true }),
        ),
      )
      expect(box).toBeChecked()
    })

    it('puts the box back and shows why when the save is refused', async () => {
      vi.mocked(App.SaveSettings).mockRejectedValueOnce('the settings file is read-only')
      useSettingsStore.setState({
        settings: { ...DEFAULT_SETTINGS, loadingSplashAvailable: true, loadingSplashOn: true },
        loaded: true,
        error: null,
      })
      render(<SettingsPanel onClose={() => undefined} onShowChapter={() => undefined} />)
      fireEvent.click(screen.getByRole('switch', { name: 'Loading splash' }))
      expect(await screen.findByText('the settings file is read-only')).toBeInTheDocument()
      expect(screen.getByRole('switch', { name: 'Loading splash' })).toBeChecked()
    })
  })
})

describe('SettingsPanel previews', () => {
  // The section is its own chunk; loading it first keeps the test off the transform.
  beforeAll(async () => {
    await import('./PreviewSection')
  }, 60_000)

  beforeEach(() => {
    vi.resetAllMocks()
    Object.assign(window, { go: {} })
    vi.mocked(App.GetPreviewSituations).mockResolvedValue([
      {
        id: 'crashed',
        label: 'Crashed, with a crash report',
        scope: 'chapter',
        card: true,
        playsInstall: false,
      },
    ] as never)
    vi.mocked(App.StartPreview).mockResolvedValue({ cardSkipped: false })
    vi.mocked(App.GetEngine).mockResolvedValue(engine)
    vi.mocked(App.GetInstances).mockResolvedValue({
      root: '',
      dir: '',
      present: {},
      packUrl: {},
      sizeBytes: {},
    })
    vi.mocked(App.GetPackStates).mockResolvedValue([])
    useSettingsStore.setState({ settings: DEFAULT_SETTINGS, loaded: true, error: null })
    useEngineStore.setState({ engine, instances: null })
    useChapterStore.setState({ selectedId: 'luxemburg' })
  })
  afterEach(() => {
    cleanup()
    Reflect.deleteProperty(window, 'go')
  })

  it('has the previews under Developer, and a start brings the chapter up on its main view', async () => {
    const onShowChapter = vi.fn()
    render(<SettingsPanel onClose={() => undefined} onShowChapter={onShowChapter} />)
    expect(screen.getByRole('heading', { name: 'Developer' })).toBeInTheDocument()
    // Closed until its title is clicked: nothing of it is on the page.
    const toggle = screen.getByRole('button', { name: 'Developer' })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByLabelText(/Local pack for/)).toBeNull()
    fireEvent.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByLabelText('Local pack for Luxemburg')).toBeInTheDocument()
    fireEvent.click(await screen.findByRole('button', { name: 'Crashed, with a crash report' }))
    await waitFor(() => expect(onShowChapter).toHaveBeenCalledExactlyOnceWith('luxemburg'))
    expect(App.StartPreview).toHaveBeenCalledExactlyOnceWith('luxemburg', 'crashed')
  })
})
