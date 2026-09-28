import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { models } from '../wailsjs/go/models'
import * as Bindings from '../wailsjs/go/main/App'
import App from './App'
import { useChapterStore } from './stores/useChapterStore'
import { useEngineStore } from './stores/useEngineStore'
import { DEFAULT_SETTINGS, useSettingsStore } from './stores/useSettingsStore'
import { BUNDLED_MANIFEST } from './lib/manifest'

vi.mock('../wailsjs/go/main/App')

describe('App', () => {
  beforeEach(() => {
    useChapterStore.setState({ manifest: BUNDLED_MANIFEST, selectedId: 'luxemburg', loaded: false })
    useEngineStore.setState({ engine: null, launching: null, error: null })
    useSettingsStore.setState({ settings: DEFAULT_SETTINGS, loaded: false, error: null })
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

  it('shows the disclaimer the usage guidelines require', () => {
    render(<App />)
    expect(screen.getByText(/NOT AN OFFICIAL MINECRAFT PRODUCT/)).toBeInTheDocument()
  })
})
