import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as App from '../../wailsjs/go/main/App'
import { DEFAULT_SETTINGS, sanitize, useSettingsStore } from './useSettingsStore'
import type { AppSettings } from '../types'

vi.mock('../../wailsjs/go/main/App')

// jsdom has no window.go, so the default here is the no-bridge preview case.
const attachBridge = () => Object.assign(window, { go: {} })
const detachBridge = () => {
  delete (window as unknown as { go?: unknown }).go
}

describe('useSettingsStore', () => {
  beforeEach(() => {
    useSettingsStore.setState({ settings: DEFAULT_SETTINGS, loaded: false, error: null })
    vi.mocked(App.GetSettings).mockReset()
    vi.mocked(App.SaveSettings).mockReset()
  })
  afterEach(detachBridge)

  it('loads and sanitizes what the backend holds', async () => {
    vi.mocked(App.GetSettings).mockResolvedValue({
      theme: 'sepia',
      profileName: 'S',
    } as AppSettings)
    await useSettingsStore.getState().load()
    const s = useSettingsStore.getState().settings
    expect(s.theme).toBe('dark')
    expect(s.profileName).toBe('S')
    expect(s.prismRoot).toBe('')
  })

  it('keeps an optimistic update without a bridge', async () => {
    vi.mocked(App.SaveSettings).mockImplementation(() => {
      throw new TypeError('no bridge')
    })
    await useSettingsStore.getState().update({ theme: 'light' })
    expect(useSettingsStore.getState().settings.theme).toBe('light')
    expect(useSettingsStore.getState().error).toBeNull()
  })

  it('reverts, records and rethrows a real backend rejection', async () => {
    attachBridge()
    vi.mocked(App.SaveSettings).mockRejectedValue('settings: prism root must be absolute')
    await expect(useSettingsStore.getState().update({ prismRoot: 'prism' })).rejects.toBeDefined()
    expect(useSettingsStore.getState().settings.prismRoot).toBe('')
    expect(useSettingsStore.getState().error).toContain('absolute')
  })

  it('sanitize falls back to the dark theme', () => {
    expect(sanitize({ theme: 'system' } as AppSettings).theme).toBe('system')
    expect(sanitize({ theme: 'x' } as AppSettings).theme).toBe('dark')
  })
})
