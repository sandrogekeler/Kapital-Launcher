import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import * as App from '../../../wailsjs/go/main/App'
import { useEngineStore } from '../../stores/useEngineStore'
import { useGameStore } from '../../stores/useGameStore'
import { DEFAULT_SETTINGS, useSettingsStore } from '../../stores/useSettingsStore'
import type { GameState } from '../../types'
import { ProfileNotice } from './ProfileNotice'

vi.mock('../../../wailsjs/go/main/App')

const failed = (over: Partial<GameState> = {}): GameState => ({
  chapterId: 'frangfurd',
  phase: 'failed',
  reason: 'profile',
  since: '2026-10-17T10:00:05Z',
  startedAt: '2026-10-17T10:00:00Z',
  ...over,
})

describe('ProfileNotice', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.assign(window, { go: {} })
    vi.mocked(App.SaveSettings).mockResolvedValue()
    vi.mocked(App.LaunchChapter).mockResolvedValue()
    useEngineStore.setState({ error: null, launching: null })
    useSettingsStore.setState({
      settings: { ...DEFAULT_SETTINGS, profileName: 'Snadrochka' },
      loaded: true,
      error: null,
    })
    useGameStore.setState({ states: { frangfurd: failed() } })
  })
  afterEach(() => {
    cleanup()
    Reflect.deleteProperty(window, 'go')
  })

  it('says which name Prism did not know', () => {
    render(<ProfileNotice onOpenAccount={() => undefined} />)
    expect(screen.getByRole('dialog', { name: 'Prism has no account by that name' })).toBeVisible()
    expect(screen.getByText('Snadrochka')).toBeInTheDocument()
  })

  it("clears the name and plays again with Prism's default account", async () => {
    render(<ProfileNotice onOpenAccount={() => undefined} />)
    fireEvent.click(screen.getByRole('button', { name: /Play with Prism's default account/ }))
    await waitFor(() => expect(App.LaunchChapter).toHaveBeenCalledWith('frangfurd'))
    expect(App.SaveSettings).toHaveBeenCalledWith(expect.objectContaining({ profileName: '' }))
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('opens the account page and stays away for that run', () => {
    const onOpenAccount = vi.fn()
    render(<ProfileNotice onOpenAccount={onOpenAccount} />)
    fireEvent.click(screen.getByRole('button', { name: 'Open account settings' }))
    expect(onOpenAccount).toHaveBeenCalledOnce()
    expect(screen.queryByRole('dialog')).toBeNull()
    // The next run that fails the same way brings it back.
    act(() =>
      useGameStore.setState({ states: { frangfurd: failed({ since: '2026-10-17T10:05:05Z' }) } }),
    )
    expect(screen.getByRole('dialog')).toBeVisible()
  })

  it('keeps the popup and the name when the save is refused', async () => {
    vi.mocked(App.SaveSettings).mockRejectedValueOnce('settings: disk full')
    render(<ProfileNotice onOpenAccount={() => undefined} />)
    fireEvent.click(screen.getByRole('button', { name: /Play with Prism's default account/ }))
    expect(await screen.findByText('settings: disk full')).toBeInTheDocument()
    expect(App.LaunchChapter).not.toHaveBeenCalled()
    expect(useSettingsStore.getState().settings.profileName).toBe('Snadrochka')
  })

  it('shows nothing for any other end of a run, or once the name is cleared', () => {
    useGameStore.setState({ states: { frangfurd: failed({ reason: 'launch' }) } })
    render(<ProfileNotice onOpenAccount={() => undefined} />)
    expect(screen.queryByRole('dialog')).toBeNull()
    act(() => {
      useGameStore.setState({ states: { frangfurd: failed() } })
      useSettingsStore.setState({ settings: { ...DEFAULT_SETTINGS, profileName: '' } })
    })
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})
