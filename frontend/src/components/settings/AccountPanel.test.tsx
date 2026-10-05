import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import * as App from '../../../wailsjs/go/main/App'
import { usePlayerStore } from '../../stores/usePlayerStore'
import { DEFAULT_SETTINGS, useSettingsStore } from '../../stores/useSettingsStore'
import { AccountPanel } from './AccountPanel'

vi.mock('../../../wailsjs/go/main/App')

const FOUND = {
  name: 'Snadrochka',
  uuid: '069a79f4-44e9-4726-a5be-fca90e38aaf5',
  faceSrc: '/mojang-face/069a79f444e94726a5befca90e38aaf5.png?v=1',
  status: 'found',
} as const
const NOT_FOUND = { name: '', uuid: '', faceSrc: '', status: 'not_found' } as const

describe('AccountPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // A real backend answers: rejections revert and show (.claude/rules/ipc.md).
    Object.assign(window, { go: {} })
    vi.mocked(App.SaveSettings).mockResolvedValue()
    usePlayerStore.getState().clear()
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

  describe('the Mojang lookup', () => {
    it('shows the face, the UUID and "Found on Mojang" for a profile Mojang knows', async () => {
      vi.mocked(App.GetPlayerProfile).mockResolvedValue(FOUND as never)
      render(<AccountPanel onClose={() => undefined} />)
      const face = await screen.findByRole('img', { name: "Snadrochka's skin" })
      expect(face).toHaveAttribute('src', FOUND.faceSrc)
      expect(screen.queryByText('S', { selector: 'span' })).toBeNull()
      expect(screen.getByText(FOUND.uuid)).toBeInTheDocument()
      expect(screen.getByRole('status')).toHaveTextContent('Found on Mojang')
      expect(App.GetPlayerProfile).toHaveBeenCalledExactlyOnceWith('Snadrochka')
    })

    it('copies the UUID through Go and opens the skin page through OpenExternal', async () => {
      vi.mocked(App.GetPlayerProfile).mockResolvedValue(FOUND as never)
      vi.mocked(App.CopyPlayerUUID).mockResolvedValueOnce()
      vi.mocked(App.OpenExternal).mockResolvedValueOnce()
      render(<AccountPanel onClose={() => undefined} />)
      fireEvent.click(await screen.findByRole('button', { name: 'Copy UUID' }))
      await waitFor(() => expect(App.CopyPlayerUUID).toHaveBeenCalledOnce())
      expect(await screen.findByRole('button', { name: 'UUID copied' })).toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: 'Change skin' }))
      expect(App.OpenExternal).toHaveBeenCalledExactlyOnceWith(
        'https://www.minecraft.net/msaprofile/mygames/editskin',
      )
    })

    it('says so when Go cannot copy the UUID', async () => {
      vi.mocked(App.GetPlayerProfile).mockResolvedValue(FOUND as never)
      vi.mocked(App.CopyPlayerUUID).mockRejectedValueOnce('copy UUID: clipboard busy')
      render(<AccountPanel onClose={() => undefined} />)
      fireEvent.click(await screen.findByRole('button', { name: 'Copy UUID' }))
      expect(await screen.findByText('copy UUID: clipboard busy')).toBeInTheDocument()
    })

    it('says no profile has a name Mojang does not know, and keeps the initials', async () => {
      vi.mocked(App.GetPlayerProfile).mockResolvedValue(NOT_FOUND as never)
      render(<AccountPanel onClose={() => undefined} />)
      expect(await screen.findByText('No Minecraft profile has this name')).toBeInTheDocument()
      expect(screen.getByText('S', { selector: 'span' })).toBeInTheDocument()
      expect(screen.queryByRole('img')).toBeNull()
      expect(screen.queryByText('Found on Mojang')).toBeNull()
    })

    it('says nothing, and keeps the initials, when Mojang cannot be reached', async () => {
      vi.mocked(App.GetPlayerProfile).mockResolvedValue({
        name: '',
        uuid: '',
        faceSrc: '',
        status: 'unknown',
      } as never)
      render(<AccountPanel onClose={() => undefined} />)
      await waitFor(() => expect(App.GetPlayerProfile).toHaveBeenCalled())
      expect(screen.queryByText('Found on Mojang')).toBeNull()
      expect(screen.queryByText('No Minecraft profile has this name')).toBeNull()
      expect(screen.getByText('S', { selector: 'span' })).toBeInTheDocument()
    })

    it('falls back to the initials when the face does not load', async () => {
      vi.mocked(App.GetPlayerProfile).mockResolvedValue(FOUND as never)
      render(<AccountPanel onClose={() => undefined} />)
      fireEvent.error(await screen.findByRole('img', { name: "Snadrochka's skin" }))
      expect(screen.getByText('S', { selector: 'span' })).toBeInTheDocument()
      expect(screen.queryByRole('img')).toBeNull()
    })

    it('asks once when the page opens, not per keystroke, and again after a save', async () => {
      vi.mocked(App.GetPlayerProfile).mockResolvedValue(FOUND as never)
      render(<AccountPanel onClose={() => undefined} />)
      await screen.findByText('Found on Mojang')
      const field = screen.getByLabelText('Profile name')
      fireEvent.change(field, { target: { value: 'Not' } })
      fireEvent.change(field, { target: { value: 'Notch' } })
      expect(App.GetPlayerProfile).toHaveBeenCalledTimes(1)
      fireEvent.keyDown(field, { key: 'Enter' })
      await waitFor(() => expect(App.GetPlayerProfile).toHaveBeenCalledTimes(2))
      expect(App.GetPlayerProfile).toHaveBeenLastCalledWith('Notch')
    })

    it('asks nothing for the default account', async () => {
      useSettingsStore.setState({ settings: { ...DEFAULT_SETTINGS, profileName: '' } })
      render(<AccountPanel onClose={() => undefined} />)
      await screen.findByText("Prism's default account", { selector: 'span' })
      expect(App.GetPlayerProfile).not.toHaveBeenCalled()
    })
  })
})
