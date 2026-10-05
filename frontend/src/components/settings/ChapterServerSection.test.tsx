import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import * as App from '../../../wailsjs/go/main/App'
import { DEFAULT_SETTINGS, useSettingsStore } from '../../stores/useSettingsStore'
import { BUNDLED_MANIFEST } from '../../lib/manifest'
import { ChapterServerSection } from './ChapterServerSection'

vi.mock('../../../wailsjs/go/main/App')

const [luxemburg, lichdenstein, frangfurd] = BUNDLED_MANIFEST.chapters as [
  (typeof BUNDLED_MANIFEST.chapters)[number],
  (typeof BUNDLED_MANIFEST.chapters)[number],
  (typeof BUNDLED_MANIFEST.chapters)[number],
]
const JOIN = 'Join the server on Play'

describe('The Server section of a chapter settings page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // A real backend answers: rejections revert and show (.claude/rules/ipc.md).
    Object.assign(window, { go: {} })
    vi.mocked(App.SaveSettings).mockResolvedValue()
    useSettingsStore.setState({ settings: DEFAULT_SETTINGS, loaded: true, error: null })
  })
  afterEach(() => {
    cleanup()
    Reflect.deleteProperty(window, 'go')
  })

  it('offers a chapter with more than one address a choice of its labels, and the switch, off', () => {
    render(<ChapterServerSection chapter={frangfurd} />)
    expect(screen.getByRole('heading', { name: 'Server' })).toBeInTheDocument()
    const group = screen.getByRole('radiogroup', { name: 'Address' })
    expect([...group.querySelectorAll('[role="radio"]')].map((r) => r.textContent)).toEqual([
      'Global',
      'Germany',
    ])
    // The default, the first, is on until the player picks.
    expect(screen.getByRole('radio', { name: 'Global' })).toBeChecked()
    expect(screen.getByRole('radio', { name: 'Germany' })).not.toBeChecked()
    expect(screen.getByRole('switch', { name: JOIN })).not.toBeChecked()
  })

  it('shows the one address of a chapter as text, with the switch', () => {
    render(<ChapterServerSection chapter={lichdenstein} />)
    expect(screen.queryByRole('radiogroup')).toBeNull()
    expect(screen.getByText('Address')).toBeInTheDocument()
    expect(screen.getByText('Main')).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: JOIN })).not.toBeChecked()
    // Never the address itself.
    expect(document.body.textContent).not.toContain(lichdenstein.server!.addresses[0]!.address)
  })

  it('renders nothing for a chapter with no server', () => {
    const { container } = render(<ChapterServerSection chapter={{ ...luxemburg, server: null }} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('saves the label on click, and follows the saved choice', async () => {
    render(<ChapterServerSection chapter={frangfurd} />)
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
    render(<ChapterServerSection chapter={frangfurd} />)
    expect(screen.getByRole('radio', { name: 'Germany' })).toBeChecked()
    cleanup()
    useSettingsStore.setState({
      settings: { ...DEFAULT_SETTINGS, serverChoices: { frangfurd: 'Asia' } },
    })
    render(<ChapterServerSection chapter={frangfurd} />)
    expect(screen.getByRole('radio', { name: 'Global' })).toBeChecked()
  })

  it('puts the address back and shows the refusal under the control', async () => {
    vi.mocked(App.SaveSettings).mockRejectedValue(
      new Error('settings: server choice "Germany" is not one of frangfurd\'s addresses'),
    )
    render(<ChapterServerSection chapter={frangfurd} />)
    fireEvent.click(screen.getByRole('radio', { name: 'Germany' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(/is not one of frangfurd/)
    await act(async () => undefined)
    expect(screen.getByRole('radio', { name: 'Global' })).toBeChecked()
    expect(useSettingsStore.getState().settings.serverChoices).toBeUndefined()
  })

  it('saves the switch on and off by chapter id, keeping the other chapters', async () => {
    useSettingsStore.setState({
      settings: { ...DEFAULT_SETTINGS, joinServers: ['lichdenstein'] },
    })
    render(<ChapterServerSection chapter={frangfurd} />)
    expect(screen.getByRole('switch', { name: JOIN })).not.toBeChecked()
    fireEvent.click(screen.getByRole('switch', { name: JOIN }))
    await waitFor(() =>
      expect(App.SaveSettings).toHaveBeenLastCalledWith(
        expect.objectContaining({ joinServers: ['lichdenstein', 'frangfurd'] }),
      ),
    )
    expect(screen.getByRole('switch', { name: JOIN })).toBeChecked()

    fireEvent.click(screen.getByRole('switch', { name: JOIN }))
    await waitFor(() =>
      expect(App.SaveSettings).toHaveBeenLastCalledWith(
        expect.objectContaining({ joinServers: ['lichdenstein'] }),
      ),
    )
    expect(screen.getByRole('switch', { name: JOIN })).not.toBeChecked()
  })

  it('puts the switch back and shows the refusal under it', async () => {
    vi.mocked(App.SaveSettings).mockRejectedValue(
      new Error('settings: join switch for "frangfurd": no such chapter with a server'),
    )
    render(<ChapterServerSection chapter={frangfurd} />)
    fireEvent.click(screen.getByRole('switch', { name: JOIN }))
    expect(await screen.findByRole('alert')).toHaveTextContent(/no such chapter with a server/)
    await act(async () => undefined)
    expect(screen.getByRole('switch', { name: JOIN })).not.toBeChecked()
    expect(useSettingsStore.getState().settings.joinServers).toBeUndefined()
  })

  it('clears a refusal once a later save goes through', async () => {
    vi.mocked(App.SaveSettings).mockRejectedValueOnce(new Error('settings: refused'))
    render(<ChapterServerSection chapter={frangfurd} />)
    fireEvent.click(screen.getByRole('switch', { name: JOIN }))
    expect(await screen.findByRole('alert')).toHaveTextContent('settings: refused')
    fireEvent.click(screen.getByRole('switch', { name: JOIN }))
    await waitFor(() => expect(screen.queryByRole('alert')).toBeNull())
    expect(screen.getByRole('switch', { name: JOIN })).toBeChecked()
  })

  it('holds a place for the Distant Horizons world data where the pack has it (issue 136)', () => {
    render(<ChapterServerSection chapter={frangfurd} />)
    expect(screen.getByText('World data')).toBeInTheDocument()
    expect(screen.getByText('Not available yet')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Download' })).toBeDisabled()
    cleanup()
    render(<ChapterServerSection chapter={luxemburg} />)
    expect(screen.queryByText('World data')).toBeNull()
  })
})
