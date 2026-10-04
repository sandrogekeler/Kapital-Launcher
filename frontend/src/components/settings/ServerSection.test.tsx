import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import * as App from '../../../wailsjs/go/main/App'
import { DEFAULT_SETTINGS, useSettingsStore } from '../../stores/useSettingsStore'
import { useChapterStore } from '../../stores/useChapterStore'
import { useEngineStore } from '../../stores/useEngineStore'
import { BUNDLED_MANIFEST } from '../../lib/manifest'
import type { Manifest } from '../../types'
import { SettingsPanel } from './SettingsPanel'

vi.mock('../../../wailsjs/go/main/App')

/** The bundled manifest with Frangfurd's server cut down to its first address. */
const oneAddressEach: Manifest = {
  ...BUNDLED_MANIFEST,
  chapters: BUNDLED_MANIFEST.chapters.map((c) =>
    c.server ? { ...c, server: { ...c.server, addresses: c.server.addresses.slice(0, 1) } } : c,
  ),
}

const open = () =>
  render(<SettingsPanel onClose={() => undefined} onShowChapter={() => undefined} />)

describe('The Server section of the settings screen', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // A real backend answers: rejections revert and show (.claude/rules/ipc.md).
    Object.assign(window, { go: {} })
    vi.mocked(App.SaveSettings).mockResolvedValue()
    useSettingsStore.setState({ settings: DEFAULT_SETTINGS, loaded: true, error: null })
    useChapterStore.setState({ manifest: BUNDLED_MANIFEST })
    useEngineStore.setState({ engine: null, instances: null })
  })
  afterEach(() => {
    cleanup()
    Reflect.deleteProperty(window, 'go')
  })

  it('offers each chapter with more than one address a choice of its labels, and no other', () => {
    open()
    expect(screen.getByRole('heading', { name: 'Server' })).toBeInTheDocument()
    expect(
      screen.getByText('The launcher checks and joins the chosen address.'),
    ).toBeInTheDocument()
    const group = screen.getByRole('radiogroup', { name: 'Server for Frangfurd' })
    const radios = group.querySelectorAll('[role="radio"]')
    expect([...radios].map((r) => r.textContent)).toEqual(['Global', 'Germany'])
    // The default, the first, is the one on until the player picks.
    expect(screen.getByRole('radio', { name: 'Global' })).toBeChecked()
    expect(screen.getByRole('radio', { name: 'Germany' })).not.toBeChecked()
    // Lichdenstein's one address is not a choice.
    expect(screen.queryByRole('radiogroup', { name: /Lichdenstein/ })).toBeNull()
    expect(screen.queryByRole('radiogroup', { name: /Luxemburg/ })).toBeNull()
  })

  it('does not render when no chapter has two addresses', () => {
    useChapterStore.setState({ manifest: oneAddressEach })
    open()
    expect(screen.queryByRole('heading', { name: 'Server' })).toBeNull()
    expect(screen.queryByText('The launcher checks and joins the chosen address.')).toBeNull()
  })

  it('saves the label on click, and follows the saved choice', async () => {
    open()
    fireEvent.click(screen.getByRole('radio', { name: 'Germany' }))
    await waitFor(() =>
      expect(App.SaveSettings).toHaveBeenCalledWith(
        expect.objectContaining({ serverChoices: { frangfurd: 'Germany' } }),
      ),
    )
    expect(screen.getByRole('radio', { name: 'Germany' })).toBeChecked()
    // What is saved is a label, never an address.
    const saved = vi.mocked(App.SaveSettings).mock.calls[0]![0]
    expect(JSON.stringify(saved.serverChoices)).not.toContain('.')
  })

  it('shows the saved choice, and falls back on the first for a label no longer listed', () => {
    useSettingsStore.setState({
      settings: { ...DEFAULT_SETTINGS, serverChoices: { frangfurd: 'Germany' } },
    })
    open()
    expect(screen.getByRole('radio', { name: 'Germany' })).toBeChecked()
    cleanup()
    useSettingsStore.setState({
      settings: { ...DEFAULT_SETTINGS, serverChoices: { frangfurd: 'Asia' } },
    })
    open()
    expect(screen.getByRole('radio', { name: 'Global' })).toBeChecked()
  })

  it('puts the choice back and shows the refusal under the control', async () => {
    vi.mocked(App.SaveSettings).mockRejectedValue(
      new Error('settings: server choice "Germany" is not one of frangfurd\'s addresses'),
    )
    open()
    fireEvent.click(screen.getByRole('radio', { name: 'Germany' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(/is not one of frangfurd/)
    await act(async () => undefined)
    expect(screen.getByRole('radio', { name: 'Global' })).toBeChecked()
    expect(useSettingsStore.getState().settings.serverChoices).toBeUndefined()
  })
})
