import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import * as App from '../../../wailsjs/go/main/App'
import { DEFAULT_SETTINGS, useSettingsStore } from '../../stores/useSettingsStore'
import { AccountPanel } from './AccountPanel'

vi.mock('../../../wailsjs/go/main/App')

describe('AccountPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // A real backend answers: rejections revert and show (.claude/rules/ipc.md).
    Object.assign(window, { go: {} })
    vi.mocked(App.SaveSettings).mockResolvedValue()
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
