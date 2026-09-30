import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { models } from '../wailsjs/go/models'
import * as Bindings from '../wailsjs/go/main/App'
import App from './App'
import { useChapterStore } from './stores/useChapterStore'
import { useEngineStore } from './stores/useEngineStore'
import { DEFAULT_SETTINGS, useSettingsStore } from './stores/useSettingsStore'
import { useServerStore } from './stores/useServerStore'
import { BUNDLED_MANIFEST } from './lib/manifest'

vi.mock('../wailsjs/go/main/App')

/** The bundled manifest with Frangfurd's pack hosted, as #25 will make it. */
function withFrangfurdHosted() {
  return {
    ...BUNDLED_MANIFEST,
    chapters: BUNDLED_MANIFEST.chapters.map((c) =>
      c.id === 'frangfurd'
        ? {
            ...c,
            pack: { ...c.pack, packwiz: 'https://kapitel-kapital.pages.dev/frangfurd/pack.toml' },
          }
        : c,
    ),
  }
}

const prismFound = {
  found: true,
  executable: 'C:/Prism/prismlauncher.exe',
  version: '11.1.0',
  root: '',
  source: 'standard-location',
}

const report = (present: Record<string, boolean>, sizeBytes: Record<string, number> = {}) =>
  models.InstanceReport.createFrom({
    root: 'C:/Prism',
    dir: 'C:/Prism/instances',
    present,
    sizeBytes,
  })

/** The bundled manifest with Lichdenstein's server at a settled address. */
function withLichdensteinAt(address: string) {
  return {
    ...BUNDLED_MANIFEST,
    chapters: BUNDLED_MANIFEST.chapters.map((c) =>
      c.id === 'lichdenstein' && c.server ? { ...c, server: { ...c.server, address } } : c,
    ),
  }
}

/**
 * Picks a chapter in the nav and ends the leaving card's slide at once (#59):
 * jsdom runs no animations, so the card that is leaving would otherwise stay
 * mounted, inert, with a second copy of every text.
 */
function switchTo(name: RegExp) {
  fireEvent.click(screen.getByRole('button', { name }))
  const leaving = document.querySelector('[inert]')
  // jsdom has no AnimationEvent, so React listens for the prefixed name there.
  if (leaving) fireEvent(leaving, new Event('webkitAnimationEnd', { bubbles: true }))
}

describe('App', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useChapterStore.setState({
      manifest: BUNDLED_MANIFEST,
      selectedId: 'luxemburg',
      loaded: false,
      wikiPages: [],
      wikiPick: {},
    })
    useEngineStore.setState({
      engine: null,
      instances: null,
      launching: null,
      installing: null,
      installedNow: null,
      error: null,
      release: null,
      install: null,
    })
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
    vi.mocked(Bindings.GetInstances).mockRejectedValue('no root')
    vi.mocked(Bindings.GetPrismRelease).mockRejectedValue('offline')
    vi.mocked(Bindings.OpenExternal).mockResolvedValue()
    vi.mocked(Bindings.GetSettings).mockResolvedValue(DEFAULT_SETTINGS)
    vi.mocked(Bindings.SaveSettings).mockResolvedValue()
    vi.mocked(Bindings.GetWikiPages).mockResolvedValue([])
  })
  afterEach(cleanup)

  it('renders every chapter and sets the accent from the open one', async () => {
    render(<App />)
    expect(await screen.findByRole('heading', { level: 1 })).toHaveAccessibleName('Luxemburg')
    const nav = within(screen.getByRole('navigation', { name: 'Chapters' }))
    expect(nav.getAllByRole('button')).toHaveLength(3)
    expect(document.documentElement.dataset.chapter).toBe('luxemburg')

    switchTo(/03.*Frangfurd/)
    expect(screen.getByRole('heading', { level: 1 })).toHaveAccessibleName('Frangfurd')
    expect(document.documentElement.dataset.chapter).toBe('frangfurd')
    expect(screen.getByText('NeoForge 1.21.1')).toBeInTheDocument()
  })

  it('says Join for a server chapter and disables Play while Prism is missing', async () => {
    render(<App />)
    await screen.findByRole('heading', { level: 1 })
    switchTo(/02.*Lichdenstein/)
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
    // A settled address: a placeholder one never reads as online.
    const manifest = withLichdensteinAt('play.example')
    useChapterStore.setState({ manifest })
    vi.mocked(Bindings.GetManifest).mockResolvedValue(models.Manifest.createFrom(manifest))
    render(<App />)
    await screen.findByRole('heading', { level: 1 })
    switchTo(/02.*Lichdenstein/)
    expect(await screen.findByText('● Server online')).toBeInTheDocument()
    expect(screen.getByText(/2\/20 players/)).toBeInTheDocument()
    // The server's version is on the state line now; the facts are the pack's (#57).
    expect(screen.queryByText('Runs')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Join Lichdenstein' })).toBeEnabled()

    // Frangfurd has a server too, but is played as a pack.
    switchTo(/03.*Frangfurd/)
    expect(screen.getByRole('button', { name: 'Play Frangfurd' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Check the server now' })).toBeInTheDocument()
  })

  it('shows an unsettled server address as pending, not as the placeholder host', async () => {
    vi.mocked(Bindings.GetEngine).mockResolvedValue({
      found: true,
      executable: '/usr/bin/prismlauncher',
      version: '11.1.0',
      root: '',
      source: 'path',
    })
    render(<App />)
    await screen.findByRole('heading', { level: 1 })
    switchTo(/02.*Lichdenstein/)
    expect(await screen.findByText('○ No server yet')).toBeInTheDocument()
    expect(screen.queryByText(/placeholder\.invalid/)).not.toBeInTheDocument()
    expect(screen.getByText('Address pending')).toBeInTheDocument()
    expect(screen.queryByText('Server')).not.toBeInTheDocument()
  })

  // Restoring the saved chapter and saving a new selection once fed each
  // other: every save re-ran detection and spawned Prism, forever (#6).
  it('reopens the saved chapter without writing it back, and saves a new pick once', async () => {
    vi.mocked(Bindings.GetSettings).mockResolvedValue({
      ...DEFAULT_SETTINGS,
      lastChapter: 'frangfurd',
    })
    render(<App />)
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1 })).toHaveAccessibleName('Frangfurd'),
    )
    // Let any effect ping-pong play out before counting.
    await new Promise((r) => setTimeout(r, 50))
    expect(Bindings.SaveSettings).not.toHaveBeenCalled()

    switchTo(/02.*Lichdenstein/)
    await waitFor(() => expect(Bindings.SaveSettings).toHaveBeenCalledTimes(1))
    await new Promise((r) => setTimeout(r, 50))
    expect(Bindings.SaveSettings).toHaveBeenCalledTimes(1)
    expect(Bindings.SaveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ lastChapter: 'lichdenstein' }),
    )
    expect(screen.getByRole('heading', { level: 1 })).toHaveAccessibleName('Lichdenstein')
  })

  it('shows Install, disabled, for a chapter whose pack is not hosted, and looks again on focus', async () => {
    vi.mocked(Bindings.GetEngine).mockResolvedValue(prismFound)
    vi.mocked(Bindings.GetInstances).mockResolvedValue(report({ luxemburg: false }))
    render(<App />)
    expect(await screen.findByText('○ Not published yet')).toBeInTheDocument()
    expect(screen.getByText('Its pack is not hosted yet')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Install Luxemburg' })).toBeDisabled()
    expect(screen.queryByRole('button', { name: 'Play Luxemburg' })).not.toBeInTheDocument()

    // The size fact waits for an instance to measure (#57).
    expect(screen.getByText('Size').nextElementSibling).toHaveTextContent('[PLACEHOLDER]')

    // Made in Prism by hand while the launcher was in the background.
    vi.mocked(Bindings.GetInstances).mockResolvedValue(
      report({ luxemburg: true }, { luxemburg: 12_300_000_000 }),
    )
    fireEvent.focus(window)
    expect(await screen.findByRole('button', { name: 'Play Luxemburg' })).toBeEnabled()
    expect(screen.queryByText('○ Not published yet')).not.toBeInTheDocument()
    expect(screen.getByText('Size').nextElementSibling).toHaveTextContent('12.3 GB')
  })

  it('keeps Play when the instance cannot be looked for', async () => {
    vi.mocked(Bindings.GetEngine).mockResolvedValue(prismFound)
    render(<App />)
    expect(await screen.findByRole('button', { name: 'Play Luxemburg' })).toBeEnabled()
    expect(screen.queryByRole('button', { name: /Install/ })).not.toBeInTheDocument()
  })

  it('installs a hosted chapter, then offers Play and says the pack comes with it', async () => {
    const manifest = withFrangfurdHosted()
    useChapterStore.setState({ manifest, selectedId: 'frangfurd' })
    vi.mocked(Bindings.GetManifest).mockResolvedValue(models.Manifest.createFrom(manifest))
    vi.mocked(Bindings.GetEngine).mockResolvedValue(prismFound)
    vi.mocked(Bindings.GetInstances).mockResolvedValue(report({ frangfurd: false }))
    let finish!: (r: models.InstanceReport) => void
    vi.mocked(Bindings.InstallChapter).mockReturnValue(new Promise((r) => (finish = r)))
    vi.mocked(Bindings.LaunchChapter).mockResolvedValue()
    render(<App />)
    expect(await screen.findByText('○ Not installed')).toBeInTheDocument()
    expect(screen.getByText('Install adds kapital-frangfurd to Prism')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Install Frangfurd' }))
    expect(Bindings.InstallChapter).toHaveBeenCalledWith('frangfurd')
    expect(await screen.findByText('◐ Installing')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Install Frangfurd' })).toBeDisabled()

    await act(async () => finish(report({ frangfurd: true })))
    expect(await screen.findByText('● Installed')).toBeInTheDocument()
    expect(screen.getByText('The first Play downloads the pack')).toBeInTheDocument()
    const play = screen.getByRole('button', { name: 'Play Frangfurd' })
    expect(play).toBeEnabled()

    fireEvent.click(play)
    expect(Bindings.LaunchChapter).toHaveBeenCalledWith('frangfurd')
    await waitFor(() => expect(screen.queryByText('● Installed')).not.toBeInTheDocument())
  })

  it('keeps Install and shows why when the install fails', async () => {
    const manifest = withFrangfurdHosted()
    useChapterStore.setState({ manifest, selectedId: 'frangfurd' })
    vi.mocked(Bindings.GetManifest).mockResolvedValue(models.Manifest.createFrom(manifest))
    vi.mocked(Bindings.GetEngine).mockResolvedValue(prismFound)
    vi.mocked(Bindings.GetInstances).mockResolvedValue(report({ frangfurd: false }))
    vi.mocked(Bindings.InstallChapter).mockRejectedValue(
      'frangfurd: the manifest says Minecraft 1.21.1, the pack 1.21.4',
    )
    render(<App />)
    fireEvent.click(await screen.findByRole('button', { name: 'Install Frangfurd' }))
    expect(
      await screen.findByText('frangfurd: the manifest says Minecraft 1.21.1, the pack 1.21.4'),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Install Frangfurd' })).toBeEnabled()
    expect(screen.getByText('○ Not installed')).toBeInTheDocument()
  })

  it('installs from a local pack named in settings, then marks the instance as a dev pack', async () => {
    useChapterStore.setState({ selectedId: 'frangfurd' })
    vi.mocked(Bindings.GetSettings).mockResolvedValue({
      ...DEFAULT_SETTINGS,
      lastChapter: 'frangfurd',
      packOverrides: { frangfurd: 'http://localhost:8080/pack.toml' },
    })
    vi.mocked(Bindings.GetEngine).mockResolvedValue(prismFound)
    vi.mocked(Bindings.GetInstances).mockResolvedValue(report({ frangfurd: false }))
    const installed = models.InstanceReport.createFrom({
      root: 'C:/Prism',
      dir: 'C:/Prism/instances',
      present: { frangfurd: true },
      packUrl: { frangfurd: 'http://localhost:8080/pack.toml' },
    })
    vi.mocked(Bindings.InstallChapter).mockResolvedValue(installed)
    vi.mocked(Bindings.LaunchChapter).mockResolvedValue()
    render(<App />)
    expect(
      await screen.findByText('Install from the dev pack at localhost:8080'),
    ).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Install Frangfurd' }))
    expect(await screen.findByText('● Installed')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Play Frangfurd' }))
    expect(await screen.findByText('● Dev pack')).toBeInTheDocument()
    expect(screen.getByText('Syncs from localhost:8080')).toBeInTheDocument()
  })

  it('offers to get Prism, says what it downloads, and follows the install', async () => {
    const release = models.PrismRelease.createFrom({
      version: '11.1.1',
      asset: 'PrismLauncher-Windows-MSVC-Portable-11.1.1.zip',
      url: 'https://github.com/PrismLauncher/PrismLauncher/releases/download/11.1.1/x.zip',
      size: 20396629,
      digest: 'sha256:ab',
      page: 'https://github.com/PrismLauncher/PrismLauncher/releases/tag/11.1.1',
      installed: '',
      updateAvailable: false,
    })
    vi.mocked(Bindings.GetPrismRelease).mockResolvedValue(release)
    render(<App />)
    await screen.findByText('Kapital Launcher can get it for you')

    fireEvent.click(screen.getByRole('button', { name: 'Get Prism Launcher' }))
    const card = screen.getByRole('region', { name: 'Get Prism Launcher' })
    expect(card).toHaveTextContent('Prism Launcher 11.1.1 (19.5 MB)')
    expect(card).toHaveTextContent('sign in with Microsoft')
    fireEvent.click(within(card).getByRole('button', { name: /What.s in 11.1.1/ }))
    expect(Bindings.OpenExternal).toHaveBeenCalledWith(release.page)
    fireEvent.click(within(card).getByRole('button', { name: 'Not now' }))
    expect(screen.queryByRole('region', { name: 'Get Prism Launcher' })).not.toBeInTheDocument()
    expect(Bindings.InstallPrism).not.toHaveBeenCalled()

    let finish!: () => void
    vi.mocked(Bindings.InstallPrism).mockReturnValue(new Promise<void>((r) => (finish = r)))
    fireEvent.click(screen.getByRole('button', { name: 'Get Prism Launcher' }))
    fireEvent.click(screen.getByRole('button', { name: 'Download and set up' }))
    expect(Bindings.InstallPrism).toHaveBeenCalledOnce()
    expect(await screen.findByText('◐ Getting Prism · 0%')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Play Luxemburg' })).toBeDisabled()

    vi.mocked(Bindings.GetEngine).mockResolvedValue({
      found: true,
      executable: 'C:/data/prism/app-11.1.1/prismlauncher.exe',
      version: '11.1.1',
      root: 'C:/data/prism/root',
      source: 'managed',
    })
    await act(async () => finish())
    expect(await screen.findByText('● Prism is ready')).toBeInTheDocument()
    expect(screen.getByText(/sign in with Microsoft/)).toBeInTheDocument()
    expect(screen.getByText('Prism 11.1.1 · managed')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Play Luxemburg' })).toBeEnabled()
  })

  it("falls back to Prism's website when the release cannot be read", async () => {
    render(<App />)
    await screen.findByText('Install Prism Launcher and sign in there')
    fireEvent.click(screen.getByRole('button', { name: 'Get Prism Launcher' }))
    expect(Bindings.OpenExternal).toHaveBeenCalledWith('https://prismlauncher.org')
    expect(screen.queryByRole('region', { name: 'Get Prism Launcher' })).not.toBeInTheDocument()
  })

  it('offers a one-click update for the managed Prism', async () => {
    vi.mocked(Bindings.GetEngine).mockResolvedValue({
      found: true,
      executable: 'C:/data/prism/app-11.1.1/prismlauncher.exe',
      version: '11.1.1',
      root: 'C:/data/prism/root',
      source: 'managed',
    })
    vi.mocked(Bindings.GetPrismRelease).mockResolvedValue(
      models.PrismRelease.createFrom({
        version: '11.2.0',
        asset: 'a.zip',
        url: 'https://github.com/x',
        size: 1,
        digest: 'sha256:ab',
        page: 'https://github.com/p',
        installed: '11.1.1',
        updateAvailable: true,
      }),
    )
    vi.mocked(Bindings.InstallPrism).mockResolvedValue()
    render(<App />)
    fireEvent.click(await screen.findByRole('button', { name: 'Update Prism to 11.2.0' }))
    expect(Bindings.InstallPrism).toHaveBeenCalledOnce()
  })

  it('shows the disclaimer the usage guidelines require', () => {
    render(<App />)
    expect(screen.getByText(/NOT AN OFFICIAL MINECRAFT PRODUCT/)).toBeInTheDocument()
  })

  it('shows a wiki page of the open chapter and opens that page, else the teaser', async () => {
    vi.mocked(Bindings.GetWikiPages).mockResolvedValue([
      {
        title: 'The Obelisk',
        line: 'A tower on the water.',
        url: 'https://kapitel-kapital.pages.dev/wiki/locations/the-obelisk',
        eras: ['Frangfurd'],
      },
    ])
    vi.mocked(Bindings.OpenWikiPage).mockResolvedValue()
    vi.mocked(Bindings.OpenChapterWiki).mockResolvedValue()
    render(<App />)
    await screen.findByRole('heading', { level: 1 })
    // Luxemburg has no page in the export: the manifest's teaser stands.
    expect(screen.getByText('Bellum Castle')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Read the history →' }))
    expect(Bindings.OpenChapterWiki).toHaveBeenCalledWith('luxemburg')

    switchTo(/03.*Frangfurd/)
    expect(await screen.findByText('The Obelisk')).toBeInTheDocument()
    expect(screen.getByText('A tower on the water.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Read the history →' }))
    expect(Bindings.OpenWikiPage).toHaveBeenCalledWith(
      'https://kapitel-kapital.pages.dev/wiki/locations/the-obelisk',
    )
  })
})
