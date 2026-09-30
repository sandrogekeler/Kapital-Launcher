import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import * as App from '../../../wailsjs/go/main/App'
import { DEFAULT_SETTINGS, useSettingsStore } from '../../stores/useSettingsStore'
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
    render(<SettingsPanel onClose={() => undefined} />)
    expect(screen.getByLabelText('Prism program')).toHaveAttribute('placeholder', engine.executable)
    expect(screen.getByText(/standard install location/)).toBeInTheDocument()
    expect(screen.getByLabelText('Prism data folder')).toHaveAttribute(
      'placeholder',
      "Prism's own data folder",
    )
  })

  it('saves the profile on Enter and the root on blur, then re-reads the engine', async () => {
    render(<SettingsPanel onClose={() => undefined} />)
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
    render(<SettingsPanel onClose={() => undefined} />)
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
    render(<SettingsPanel onClose={() => undefined} />)
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
    render(<SettingsPanel onClose={() => undefined} />)
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
    render(<SettingsPanel onClose={() => undefined} />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Browse' })[1]!)
    await screen.findByText(/dialog failed/)
    expect(App.SaveSettings).not.toHaveBeenCalled()
  })

  it('saves the theme on click', async () => {
    render(<SettingsPanel onClose={() => undefined} />)
    expect(screen.getByRole('button', { name: 'Dark' })).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(screen.getByRole('button', { name: 'Light' }))
    await waitFor(() =>
      expect(App.SaveSettings).toHaveBeenCalledWith(expect.objectContaining({ theme: 'light' })),
    )
    expect(screen.getByRole('button', { name: 'Light' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('writes and clears a local pack address per chapter', async () => {
    render(<SettingsPanel onClose={() => undefined} />)
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
    render(<SettingsPanel onClose={onClose} />)
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
})
