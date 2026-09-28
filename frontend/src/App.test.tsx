import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { models } from '../wailsjs/go/models'
import * as Bindings from '../wailsjs/go/main/App'
import App from './App'
import { useChapterStore } from './stores/useChapterStore'
import { useEngineStore } from './stores/useEngineStore'
import { DEFAULT_SETTINGS, useSettingsStore } from './stores/useSettingsStore'
import { useServerStore } from './stores/useServerStore'
import { BUNDLED_MANIFEST } from './lib/manifest'

vi.mock('../wailsjs/go/main/App')

describe('App', () => {
  beforeEach(() => {
    useChapterStore.setState({ manifest: BUNDLED_MANIFEST, selectedId: 'luxemburg', loaded: false })
    useEngineStore.setState({ engine: null, launching: null, error: null })
    useSettingsStore.setState({ settings: DEFAULT_SETTINGS, loaded: false, error: null })
    useServerStore.setState({ statuses: {}, checking: null, error: null })
    vi.mocked(Bindings.GetServerStatus).mockResolvedValue({
      chapterId: 'lichdenstein',
      checked: true,
      online: true,
      players: 2,
      max: 20,
      version: 'Paper 1.20.6',
      motd: '',
      latencyMs: 30,
      checkedAt: '2026-09-28T12:00:00Z',
    })
    vi.mocked(Bindings.GetManifest).mockResolvedValue(models.Manifest.createFrom(BUNDLED_MANIFEST))
    vi.mocked(Bindings.GetEngine).mockResolvedValue({
      found: false,
      executable: '',
      version: '',
      root: '',
      source: '',
    })
    vi.mocked(Bindings.GetSettings).mockResolvedValue(DEFAULT_SETTINGS)
    vi.mocked(Bindings.SaveSettings).mockResolvedValue()
  })
  afterEach(cleanup)

  it('renders every chapter and sets the accent from the open one', async () => {
    render(<App />)
    expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('Luxemburg')
    const nav = within(screen.getByRole('navigation', { name: 'Chapters' }))
    expect(nav.getAllByRole('button')).toHaveLength(3)
    expect(document.documentElement.dataset.chapter).toBe('luxemburg')

    fireEvent.click(screen.getByRole('button', { name: /03.*Frangfurd/ }))
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Frangfurd')
    expect(document.documentElement.dataset.chapter).toBe('frangfurd')
    expect(screen.getByText('NeoForge 1.21.1')).toBeInTheDocument()
  })

  it('says Join for a server chapter and disables Play while Prism is missing', async () => {
    render(<App />)
    await screen.findByRole('heading', { level: 1 })
    fireEvent.click(screen.getByRole('button', { name: /02.*Lichdenstein/ }))
    const play = screen.getByRole('button', { name: 'Join Lichdenstein' })
    expect(play).toBeDisabled()
    expect(screen.getByText('○ Prism not found')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Get Prism Launcher' })).toBeInTheDocument()
  })

  it('shows the server line once Prism is found and the ping has answered', async () => {
    vi.mocked(Bindings.GetEngine).mockResolvedValue({
      found: true,
      executable: '/usr/bin/prismlauncher',
      version: '11.1.0',
      root: '',
      source: 'path',
    })
    render(<App />)
    await screen.findByRole('heading', { level: 1 })
    fireEvent.click(screen.getByRole('button', { name: /02.*Lichdenstein/ }))
    expect(await screen.findByText('● Server online')).toBeInTheDocument()
    expect(screen.getByText(/2\/20 players/)).toBeInTheDocument()
    expect(screen.getByText('Paper 1.20.6')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Join Lichdenstein' })).toBeEnabled()

    // Frangfurd has a server too, but is played as a pack.
    fireEvent.click(screen.getByRole('button', { name: /03.*Frangfurd/ }))
    expect(screen.getByRole('button', { name: 'Play Frangfurd' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Check the server now' })).toBeInTheDocument()
  })

  it('shows the disclaimer the usage guidelines require', () => {
    render(<App />)
    expect(screen.getByText(/NOT AN OFFICIAL MINECRAFT PRODUCT/)).toBeInTheDocument()
  })
})
