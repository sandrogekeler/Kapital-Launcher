import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { models } from '../wailsjs/go/models'
import * as Bindings from '../wailsjs/go/main/App'
import App from './App'
import { useChapterStore } from './stores/useChapterStore'
import { useEngineStore } from './stores/useEngineStore'
import { DEFAULT_SETTINGS, useSettingsStore } from './stores/useSettingsStore'
import { useServerStore } from './stores/useServerStore'
import { useGameStore } from './stores/useGameStore'
import { BUNDLED_MANIFEST } from './lib/manifest'
import { SLIDE_INTERVAL_MS } from './lib/slides'

vi.mock('../wailsjs/go/main/App')
vi.mock('../wailsjs/runtime/runtime')

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

/** The bundled manifest with Luxemburg's pack not hosted, as before 1.5.0. */
function withLuxemburgUnhosted() {
  return {
    ...BUNDLED_MANIFEST,
    chapters: BUNDLED_MANIFEST.chapters.map((c) =>
      c.id === 'luxemburg' ? { ...c, pack: { ...c.pack, packwiz: null } } : c,
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

/** The bundled manifest with Luxemburg's server taken away: a chapter with none. */
function withLuxemburgServerless() {
  return {
    ...BUNDLED_MANIFEST,
    chapters: BUNDLED_MANIFEST.chapters.map((c) =>
      c.id === 'luxemburg' ? { ...c, server: null } : c,
    ),
  }
}

/** The bundled manifest with Lichdenstein's server at a settled address. */
function withLichdensteinAt(address: string) {
  return {
    ...BUNDLED_MANIFEST,
    chapters: BUNDLED_MANIFEST.chapters.map((c) =>
      c.id === 'lichdenstein' && c.server
        ? { ...c, server: { ...c.server, addresses: [{ label: 'Main', address }] } }
        : c,
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
  const leaving = document.querySelector('[class*="card-out-"]')
  if (leaving) endAnimation(leaving)
}

/** The end of an element's animation. jsdom has no AnimationEvent, so React listens for the prefixed name. */
function endAnimation(el: Element) {
  fireEvent(el, new Event('webkitAnimationEnd', { bubbles: true }))
}

describe('App', () => {
  // The pages are their own chunks, loaded when first asked for. Loading them
  // once up front keeps a test that opens one from racing the first transform.
  beforeAll(async () => {
    await Promise.all([
      import('./components/settings/SettingsPanel'),
      import('./components/settings/ChapterSettingsPanel'),
      import('./components/settings/PreviewSection'),
      import('./components/settings/PackSourceSection'),
      import('./components/main/RunReportPanel'),
      import('./components/main/GetPrism'),
      import('./components/main/ChapterPage'),
      import('./components/logs/LogsPanel'),
    ])
  }, 60_000)

  beforeEach(() => {
    vi.clearAllMocks()
    useChapterStore.setState({
      manifest: BUNDLED_MANIFEST,
      selectedId: 'luxemburg',
      loaded: false,
      wikiPages: [],
      wikiShots: [],
      slides: {},
    })
    useEngineStore.setState({
      engine: null,
      instances: null,
      changelogs: {},
      launching: null,
      installing: null,
      installedNow: null,
      error: null,
      release: null,
      install: null,
    })
    useSettingsStore.setState({ settings: DEFAULT_SETTINGS, loaded: false, error: null })
    useServerStore.setState({ statuses: {}, checking: null, error: null })
    useGameStore.setState({ states: {} })
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
    vi.mocked(Bindings.GetPackStates).mockResolvedValue([])
    vi.mocked(Bindings.GetChangelogs).mockResolvedValue([])
    vi.mocked(Bindings.GetGameStates).mockResolvedValue([])
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

  it('says Play for a server chapter until its switch is on, and disables Play while Prism is missing', async () => {
    render(<App />)
    await screen.findByRole('heading', { level: 1 })
    switchTo(/02.*Lichdenstein/)
    const play = screen.getByRole('button', { name: 'Play Lichdenstein' })
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
    expect(screen.getByRole('button', { name: 'Play Lichdenstein' })).toBeEnabled()

    // Frangfurd has a server too.
    switchTo(/03.*Frangfurd/)
    expect(screen.getByRole('main')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Check the server now' })).toBeInTheDocument()
  })

  it('reads Join for a chapter whose join switch is on in the launcher settings (issue 163)', async () => {
    vi.mocked(Bindings.GetSettings).mockResolvedValue({
      ...DEFAULT_SETTINGS,
      joinServers: ['lichdenstein'],
    })
    render(<App />)
    await screen.findByRole('heading', { level: 1 })
    switchTo(/02.*Lichdenstein/)
    expect(screen.getByRole('button', { name: 'Join Lichdenstein' })).toBeInTheDocument()
    // The switch is per chapter.
    switchTo(/03.*Frangfurd/)
    expect(screen.getByRole('button', { name: 'Play Frangfurd' })).toBeInTheDocument()
  })

  it("turns Play into Join from the chapter's own settings page, and back (issue 163)", async () => {
    render(<App />)
    await screen.findByRole('heading', { level: 1 })
    switchTo(/02.*Lichdenstein/)
    expect(screen.getByRole('button', { name: 'Play Lichdenstein' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Lichdenstein settings' }))
    await screen.findByRole('region', { name: 'Lichdenstein settings' })
    const join = await screen.findByRole('switch', { name: 'Join the server on Play' })
    expect(join).not.toBeChecked()
    fireEvent.click(join)
    await waitFor(() =>
      expect(Bindings.SaveSettings).toHaveBeenCalledWith(
        expect.objectContaining({ joinServers: ['lichdenstein'] }),
      ),
    )
    expect(screen.getByRole('switch', { name: 'Join the server on Play' })).toBeChecked()
    // The chapter card behind the page follows the saved switch.
    expect(
      screen.getByRole('button', { name: 'Join Lichdenstein', hidden: true }),
    ).toBeInTheDocument()

    fireEvent.click(screen.getByRole('switch', { name: 'Join the server on Play' }))
    await waitFor(() => expect(useSettingsStore.getState().settings.joinServers).toEqual([]))
    expect(
      screen.getByRole('button', { name: 'Play Lichdenstein', hidden: true }),
    ).toBeInTheDocument()
  })

  it('shows an unsettled server address as pending, not as the placeholder host', async () => {
    vi.mocked(Bindings.GetEngine).mockResolvedValue({
      found: true,
      executable: '/usr/bin/prismlauncher',
      version: '11.1.0',
      root: '',
      source: 'path',
    })
    const manifest = withLichdensteinAt('placeholder.invalid')
    useChapterStore.setState({ manifest })
    vi.mocked(Bindings.GetManifest).mockResolvedValue(models.Manifest.createFrom(manifest))
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
    const manifest = withLuxemburgUnhosted()
    useChapterStore.setState({ manifest, selectedId: 'luxemburg' })
    vi.mocked(Bindings.GetManifest).mockResolvedValue(models.Manifest.createFrom(manifest))
    vi.mocked(Bindings.GetEngine).mockResolvedValue(prismFound)
    vi.mocked(Bindings.GetInstances).mockResolvedValue(report({ luxemburg: false }))
    render(<App />)
    expect(await screen.findByText('○ Not published yet')).toBeInTheDocument()
    expect(screen.getByText('Its pack is not hosted yet')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Install Luxemburg' })).toBeDisabled()
    expect(screen.queryByRole('button', { name: 'Play Luxemburg' })).not.toBeInTheDocument()

    // The version and the size wait for an instance to measure (#57), and say
    // so in words, not as the manifest's marker.
    expect(screen.getByText('Size').nextElementSibling).toHaveTextContent('Not installed')
    expect(screen.getByText('Version').nextElementSibling).toHaveTextContent('Not installed')
    expect(screen.queryByText(/PLACEHOLDER/)).toBeNull()

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
    const card = await screen.findByRole('region', { name: 'Get Prism Launcher' })
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
    fireEvent.click(await screen.findByRole('button', { name: 'Download and set up' }))
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

  it('keeps the disclaimer the usage guidelines require to the settings screen', async () => {
    render(<App />)
    await screen.findByRole('heading', { level: 1 })
    expect(screen.queryByText(/Not an official Minecraft product/)).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Settings' }))
    expect(await screen.findByText(/Not an official Minecraft product/)).toBeInTheDocument()
  })

  it('lands a preview started in Settings on the chapter view, with its settings open beneath', async () => {
    Object.assign(window, { go: {} })
    try {
      vi.mocked(Bindings.GetPreviewSituations).mockResolvedValue([
        {
          id: 'crashed',
          label: 'Crashed, with a crash report',
          scope: 'chapter',
          card: true,
          playsInstall: false,
        },
      ] as never)
      vi.mocked(Bindings.StartPreview).mockResolvedValue({ cardSkipped: false })
      render(<App />)
      await screen.findByRole('heading', { level: 1 })
      fireEvent.click(screen.getByRole('button', { name: 'Luxemburg settings' }))
      expect(await screen.findByRole('region', { name: 'Luxemburg settings' })).toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: 'Settings' }))
      expect(await screen.findByRole('region', { name: 'Settings' })).toBeInTheDocument()

      fireEvent.click(screen.getByRole('button', { name: 'Developer' }))
      // The previews are a lazy chunk; under a full run its first load can take past a second.
      fireEvent.click(
        await screen.findByRole(
          'button',
          { name: 'Crashed, with a crash report' },
          { timeout: 5000 },
        ),
      )
      await waitFor(() => expect(Bindings.StartPreview).toHaveBeenCalledOnce())
      await waitFor(() => expect(screen.queryByRole('region', { name: 'Settings' })).toBeNull())
      // Neither page is left underneath: the app is on the chapter itself.
      expect(screen.queryByRole('region', { name: 'Luxemburg settings' })).toBeNull()
      expect(screen.getByRole('heading', { level: 1 })).toHaveAccessibleName('Luxemburg')
      expect(screen.getByRole('button', { name: 'Luxemburg settings' })).toBeInTheDocument()
    } finally {
      Reflect.deleteProperty(window, 'go')
    }
    // Two pages and a lazy chunk: under the coverage run this outlasts the default five seconds.
  }, 15_000)

  it('shows a wiki page of the open chapter and opens that page, else the teaser', async () => {
    vi.mocked(Bindings.GetWikiPages).mockResolvedValue([
      {
        title: 'The Obelisk',
        line: 'A tower on the water.',
        url: 'https://kapitel-kapital.pages.dev/wiki/locations/the-obelisk',
        eras: ['Frangfurd'],
        id: 'locations/The Obelisk.md',
        related: [],
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

  it('keeps Play reading Play when the installed pack is behind its source, and names the version', async () => {
    // The pack line is the right side's only for a chapter without a server.
    const manifest = withLuxemburgServerless()
    useChapterStore.setState({ manifest })
    vi.mocked(Bindings.GetManifest).mockResolvedValue(models.Manifest.createFrom(manifest))
    vi.mocked(Bindings.GetEngine).mockResolvedValue(prismFound)
    vi.mocked(Bindings.GetInstances).mockResolvedValue(report({ luxemburg: true }))
    vi.mocked(Bindings.LaunchChapter).mockResolvedValue()
    vi.mocked(Bindings.GetPackStates).mockResolvedValue([
      { chapterId: 'luxemburg', installed: true, checked: true, upToDate: false, version: '4.2' },
    ])
    render(<App />)
    expect(await screen.findByRole('button', { name: 'Play Luxemburg' })).toBeEnabled()
    expect(screen.queryByRole('button', { name: 'Update and play' })).toBeNull()
    expect(screen.getByText('● Updated')).toBeInTheDocument()
    // The bar's pack line and the hero's version pill (issue 162) both name it.
    expect(screen.getAllByText('Version 4.2')).toHaveLength(2)
    expect(screen.getByText('Version').nextElementSibling).toHaveTextContent('older than 4.2')
    fireEvent.click(screen.getByRole('button', { name: 'Play Luxemburg' }))
    await waitFor(() => expect(Bindings.LaunchChapter).toHaveBeenCalledWith('luxemburg'))

    // Synced: the bar reads the same, and the settings row names the version.
    vi.mocked(Bindings.GetPackStates).mockResolvedValue([
      { chapterId: 'luxemburg', installed: true, checked: true, upToDate: true, version: '4.2' },
    ])
    fireEvent.focus(window)
    await waitFor(() =>
      expect(screen.getByText('Version').nextElementSibling).toHaveTextContent(/^4\.2$/),
    )
    expect(screen.getByRole('button', { name: 'Play Luxemburg' })).toBeInTheDocument()
    expect(screen.getByText('● Updated')).toBeInTheDocument()
    // The bar's pack line and the hero's version pill (issue 162) both name it.
    expect(screen.getAllByText('Version 4.2')).toHaveLength(2)
  })

  it('fills the Changelog panel from the pack source, again when the window regains focus', async () => {
    const entry = {
      version: '4.2',
      date: '2026-10-03',
      summary: 'Removed JEI.',
      details: 'Removed JEI.\nFixed a crash.',
    }
    vi.mocked(Bindings.GetChangelogs).mockResolvedValue([
      models.PackChangelog.createFrom({ chapterId: 'luxemburg', checked: true, entries: [entry] }),
    ])
    render(<App />)
    const first = await screen.findByText('Removed JEI.')
    expect(first).toHaveAttribute('title', 'Removed JEI.\nFixed a crash.')
    expect(screen.queryByText('Fixed a crash.')).toBeNull()

    vi.mocked(Bindings.GetChangelogs).mockResolvedValue([
      models.PackChangelog.createFrom({
        chapterId: 'luxemburg',
        checked: true,
        entries: [{ ...entry, summary: 'Added a map.', details: 'Added a map.' }],
      }),
    ])
    fireEvent.focus(window)
    expect(await screen.findByText('Added a map.')).toBeInTheDocument()
  })

  it('opens the run report from Details on the notice of a game that crashed, and closes it', async () => {
    vi.mocked(Bindings.GetEngine).mockResolvedValue(prismFound)
    const crashed = {
      chapterId: 'frangfurd',
      phase: 'crashed' as const,
      since: '2026-10-02T10:00:31Z',
      startedAt: '2026-10-02T10:00:00Z',
    }
    const read = vi.fn().mockResolvedValue({
      game: crashed,
      phases: [{ phase: 'starting', ms: 0 }],
      logTail: '[Render thread/ERROR]: Reported exception thrown!\n',
      logLines: 1,
      logTruncated: false,
      crashReport: 'crash-1.txt',
      consoleAvailable: false,
    })
    const original = useGameStore.getState().report
    useGameStore.setState({ report: read })
    try {
      render(<App />)
      await screen.findByRole('heading', { level: 1 })
      switchTo(/03.*Frangfurd/)
      act(() => useGameStore.getState().receive(crashed))
      // The notice sits in the corner, outside the chapter card.
      const notice = screen.getByRole('status')
      expect(within(notice).getByText('○ The game stopped')).toBeInTheDocument()
      expect(screen.getByRole('main')).not.toHaveTextContent('○ The game stopped')

      fireEvent.click(within(notice).getByRole('button', { name: 'Details' }))
      const panel = await screen.findByRole('region', { name: 'Frangfurd run report' })
      expect(await within(panel).findByText('crash-1.txt')).toBeInTheDocument()
      expect(within(panel).getByLabelText("The end of the game's log")).toHaveTextContent(
        'Reported exception thrown!',
      )
      expect(read).toHaveBeenCalledExactlyOnceWith('frangfurd')

      fireEvent.click(within(panel).getByRole('button', { name: 'Back' }))
      expect(screen.queryByRole('region', { name: 'Frangfurd run report' })).toBeNull()
      expect(within(notice).getByRole('button', { name: 'Details' })).toBeInTheDocument()
    } finally {
      useGameStore.setState({ report: original })
    }
  })

  it("opens a chapter's logs from the console icon in its hero, and closes them with Back, Escape and a switch", async () => {
    Object.assign(window, { go: {} })
    try {
      vi.mocked(Bindings.GetEngine).mockResolvedValue(prismFound)
      vi.mocked(Bindings.GetInstances).mockResolvedValue(report({ luxemburg: true }))
      vi.mocked(Bindings.ReadRunLog).mockResolvedValue({
        kind: 'log',
        name: 'latest.log',
        text: 'a line',
        offset: 0,
        size: 7,
        lines: 1,
        truncated: false,
      } as never)
      vi.mocked(Bindings.GetRunLogs).mockResolvedValue([
        {
          kind: 'log',
          name: 'latest.log',
          modifiedAt: '2026-10-03T15:00:00Z',
          size: 10,
          crashed: false,
        },
      ] as never)
      render(<App />)
      await screen.findByRole('heading', { level: 1 })
      const logs = () => screen.queryByRole('region', { name: 'Luxemburg logs' })

      fireEvent.click(screen.getByRole('button', { name: 'Logs for Luxemburg' }))
      expect(await screen.findByRole('region', { name: 'Luxemburg logs' })).toBeInTheDocument()
      expect(await screen.findByRole('button', { name: /Choose a log/ })).toBeVisible()
      expect(Bindings.ReadRunLog).toHaveBeenCalledWith('luxemburg', 'log', 'latest.log', 0)
      expect(Bindings.GetRunLogs).toHaveBeenCalledExactlyOnceWith('luxemburg')

      fireEvent.click(screen.getByRole('button', { name: 'Back' }))
      expect(logs()).toBeNull()
      expect(screen.getByRole('button', { name: 'Logs for Luxemburg' })).toBeInTheDocument()

      fireEvent.click(screen.getByRole('button', { name: 'Logs for Luxemburg' }))
      await screen.findByRole('region', { name: 'Luxemburg logs' })
      fireEvent.keyDown(window, { key: 'Escape' })
      expect(logs()).toBeNull()

      fireEvent.click(screen.getByRole('button', { name: 'Logs for Luxemburg' }))
      await screen.findByRole('region', { name: 'Luxemburg logs' })
      switchTo(/03.*Frangfurd/)
      expect(logs()).toBeNull()
      expect(screen.getByRole('button', { name: 'Logs for Frangfurd' })).toBeInTheDocument()
    } finally {
      Reflect.deleteProperty(window, 'go')
    }
  })

  describe("a chapter's map (issue 161)", () => {
    const MAP = 'http://spiral-reminders.tun.ply.gg:1111'
    const reachable = { url: MAP, reachable: true, reason: '' }
    beforeEach(() => void Object.assign(window, { go: {} }))
    afterEach(() => void Reflect.deleteProperty(window, 'go'))

    it('opens from the map tool in the hero as a page over the chapter, framing the map once Go says it answers', async () => {
      vi.mocked(Bindings.CheckChapterMap).mockResolvedValue(reachable as never)
      render(<App />)
      await screen.findByRole('heading', { level: 1 })
      // The wiki's book is gone from the hero; the map is in its place.
      expect(screen.queryByRole('button', { name: 'Read the history on the wiki' })).toBeNull()
      switchTo(/03.*Frangfurd/)

      fireEvent.click(screen.getByRole('button', { name: 'Map of Frangfurd' }))
      const page = await screen.findByRole('region', { name: 'Frangfurd map' })
      expect(page.parentElement).toHaveClass('page-in-right')
      const frame = await within(page).findByTitle('Frangfurd map')
      expect(frame).toHaveAttribute('src', MAP)
      expect(Bindings.CheckChapterMap).toHaveBeenCalledExactlyOnceWith('frangfurd')
      expect(Bindings.OpenExternal).not.toHaveBeenCalled()

      fireEvent.click(within(page).getByRole('button', { name: 'Back' }))
      expect(screen.queryByRole('region', { name: 'Frangfurd map' })).toBeNull()
      expect(screen.getByRole('button', { name: 'Map of Frangfurd' })).toBeInTheDocument()
    })

    it('closes when another chapter is picked', async () => {
      vi.mocked(Bindings.CheckChapterMap).mockResolvedValue(reachable as never)
      render(<App />)
      await screen.findByRole('heading', { level: 1 })
      switchTo(/03.*Frangfurd/)
      fireEvent.click(screen.getByRole('button', { name: 'Map of Frangfurd' }))
      await screen.findByRole('region', { name: 'Frangfurd map' })
      switchTo(/01.*Luxemburg/)
      expect(screen.queryByRole('region', { name: 'Frangfurd map' })).toBeNull()
    })

    it("opens a chapter that has no map on a page that says so, and doesn't try the browser for it", async () => {
      vi.mocked(Bindings.CheckChapterMap).mockResolvedValue({
        url: '',
        reachable: false,
        reason: 'no map',
      } as never)
      render(<App />)
      await screen.findByRole('heading', { level: 1 })
      fireEvent.click(screen.getByRole('button', { name: 'Map of Luxemburg' }))
      const page = await screen.findByRole('region', { name: 'Luxemburg map' })
      expect(await within(page).findByText('Map could not be reached')).toBeVisible()
      expect(within(page).getByText('There is no map for Luxemburg yet.')).toBeInTheDocument()
      expect(within(page).getByRole('button', { name: 'Try again' })).toBeInTheDocument()
      expect(Bindings.OpenExternal).not.toHaveBeenCalled()
    })

    describe('with the browser chosen in settings', () => {
      beforeEach(() => {
        vi.mocked(Bindings.GetSettings).mockResolvedValue({ ...DEFAULT_SETTINGS, mapIn: 'browser' })
      })

      it("hands the chapter's map to the system browser and opens no page", async () => {
        render(<App />)
        await screen.findByRole('heading', { level: 1 })
        await waitFor(() => expect(useSettingsStore.getState().settings.mapIn).toBe('browser'))
        switchTo(/03.*Frangfurd/)

        fireEvent.click(screen.getByRole('button', { name: 'Map of Frangfurd' }))
        expect(Bindings.OpenExternal).toHaveBeenCalledExactlyOnceWith(MAP)
        expect(screen.queryByRole('region', { name: 'Frangfurd map' })).toBeNull()
        expect(Bindings.CheckChapterMap).not.toHaveBeenCalled()
      })

      it('opens the page for a chapter with no map, so the player sees why', async () => {
        vi.mocked(Bindings.CheckChapterMap).mockResolvedValue({
          url: '',
          reachable: false,
          reason: 'no map',
        } as never)
        render(<App />)
        await screen.findByRole('heading', { level: 1 })
        await waitFor(() => expect(useSettingsStore.getState().settings.mapIn).toBe('browser'))

        fireEvent.click(screen.getByRole('button', { name: 'Map of Luxemburg' }))
        const page = await screen.findByRole('region', { name: 'Luxemburg map' })
        expect(await within(page).findByText('Map could not be reached')).toBeVisible()
        expect(Bindings.OpenExternal).not.toHaveBeenCalled()
      })

      it('says nothing on screen when the browser refuses, and logs it', async () => {
        const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined)
        vi.mocked(Bindings.OpenExternal).mockRejectedValue('no default browser')
        render(<App />)
        await screen.findByRole('heading', { level: 1 })
        await waitFor(() => expect(useSettingsStore.getState().settings.mapIn).toBe('browser'))
        switchTo(/03.*Frangfurd/)
        fireEvent.click(screen.getByRole('button', { name: 'Map of Frangfurd' }))
        await waitFor(() => expect(warn).toHaveBeenCalledWith('open map', 'no default browser'))
        warn.mockRestore()
      })
    })
  })

  it('closes the run report when another chapter is picked', async () => {
    vi.mocked(Bindings.GetEngine).mockResolvedValue(prismFound)
    const crashed = {
      chapterId: 'frangfurd',
      phase: 'failed' as const,
      since: '2026-10-02T10:00:12Z',
      startedAt: '2026-10-02T10:00:00Z',
    }
    const original = useGameStore.getState().report
    useGameStore.setState({ report: vi.fn().mockResolvedValue(null) })
    try {
      render(<App />)
      await screen.findByRole('heading', { level: 1 })
      switchTo(/03.*Frangfurd/)
      act(() => useGameStore.getState().receive(crashed))
      fireEvent.click(screen.getByRole('button', { name: 'Details' }))
      await screen.findByRole('region', { name: 'Frangfurd run report' })
      fireEvent.click(screen.getByRole('button', { name: /01.*Luxemburg/ }))
      expect(screen.queryByRole('region', { name: 'Frangfurd run report' })).toBeNull()

      // Details on another chapter's notice brings that chapter and its report up.
      fireEvent.click(screen.getByRole('button', { name: 'Details' }))
      expect(
        await screen.findByRole('region', { name: 'Frangfurd run report' }),
      ).toBeInTheDocument()
      expect(useChapterStore.getState().selectedId).toBe('frangfurd')
    } finally {
      useGameStore.setState({ report: original })
    }
  })

  it('keeps the launcher as it is while a loading card is up: the card has a window of its own', async () => {
    vi.mocked(Bindings.GetEngine).mockResolvedValue(prismFound)
    render(<App />)
    await screen.findByRole('heading', { level: 1 })

    act(() =>
      useGameStore.getState().receive({
        chapterId: 'frangfurd',
        phase: 'mods',
        since: '2026-10-01T10:00:00Z',
        startedAt: '2026-10-01T09:59:00Z',
        splash: true,
      }),
    )
    expect(screen.queryByRole('region', { name: /^Loading / })).not.toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: 'Chapters' })).toBeInTheDocument()
    expect(screen.getByRole('main')).toBeInTheDocument()
  })

  it('moves the slide on every minute, from the last switch, and not with the slideshow off', async () => {
    vi.useFakeTimers()
    try {
      const advance = vi.fn(async () => undefined)
      vi.mocked(Bindings.GetWikiShots).mockResolvedValue([
        { era: 'Luxemburg', src: '/wiki-art/luxemburg/1.webp', subject: '' },
        { era: 'Luxemburg', src: '/wiki-art/luxemburg/2.webp', subject: '' },
        { era: 'Frangfurd', src: '/wiki-art/frangfurd/1.webp', subject: '' },
        { era: 'Frangfurd', src: '/wiki-art/frangfurd/2.webp', subject: '' },
      ])
      useChapterStore.setState({ advance })
      render(<App />)
      await act(() => vi.advanceTimersByTimeAsync(SLIDE_INTERVAL_MS))
      expect(advance).toHaveBeenCalledTimes(1)

      // A switch starts the minute over.
      await act(() => vi.advanceTimersByTimeAsync(SLIDE_INTERVAL_MS / 2))
      act(() => useChapterStore.getState().select('frangfurd'))
      await act(() => vi.advanceTimersByTimeAsync(SLIDE_INTERVAL_MS / 2))
      expect(advance).toHaveBeenCalledTimes(1)
      await act(() => vi.advanceTimersByTimeAsync(SLIDE_INTERVAL_MS / 2))
      expect(advance).toHaveBeenCalledTimes(2)

      act(() => useSettingsStore.setState({ settings: { ...DEFAULT_SETTINGS, staticArt: true } }))
      await act(() => vi.advanceTimersByTimeAsync(SLIDE_INTERVAL_MS * 3))
      expect(advance).toHaveBeenCalledTimes(2)
    } finally {
      vi.useRealTimers()
    }
  })

  describe('pages over the chapter', () => {
    const sliding = (cls: string) => document.querySelector<HTMLElement>(`.${cls}`)
    /** The chapter's card in the stage, which a page covers and never moves. */
    const chapterCard = () =>
      screen.getByRole('main').querySelector<HTMLElement>('.card-stage > div')!
    const prism = () => {
      vi.mocked(Bindings.GetEngine).mockResolvedValue(prismFound)
      vi.mocked(Bindings.GetInstances).mockResolvedValue(report({ luxemburg: true }))
    }
    beforeEach(() => void Object.assign(window, { go: {} }))
    afterEach(() => void Reflect.deleteProperty(window, 'go'))

    it("slides a chapter's settings in from the right inside the card, over a chapter that is left alone and inert", async () => {
      prism()
      render(<App />)
      await screen.findByRole('heading', { level: 1 })
      expect(chapterCard()).not.toHaveAttribute('inert')

      const pen = screen.getByRole('button', { name: 'Luxemburg settings' })
      act(() => pen.focus())
      fireEvent.click(pen)
      const page = await screen.findByRole('region', { name: 'Luxemburg settings' })
      // In the same stage as the card, in a frame the card's corners clip, sliding from the right.
      const layer = page.parentElement!
      expect(layer).toHaveClass('page-in-right')
      expect(layer.parentElement).toHaveClass('overflow-hidden', 'rounded-lg')
      expect(layer.parentElement!.parentElement).toBe(chapterCard().parentElement)
      expect(chapterCard()).toHaveAttribute('inert')
      expect(chapterCard()).toHaveAttribute('aria-hidden', 'true')
      expect(chapterCard()).toHaveTextContent('Play Luxemburg')
      expect(chapterCard().className).not.toMatch(/card-(in|out)|page-/)
      // The page takes the focus, and the hero behind it is not reachable.
      await waitFor(() => expect(layer).toHaveFocus())
      expect(screen.queryByRole('button', { name: 'Logs for Luxemburg' })).toBeNull()
    })

    it('keeps the page mounted while it slides out, then removes it, and returns focus to the pen', async () => {
      prism()
      render(<App />)
      await screen.findByRole('heading', { level: 1 })
      const pen = screen.getByRole('button', { name: 'Luxemburg settings' })
      act(() => pen.focus())
      fireEvent.click(pen)
      const page = await screen.findByRole('region', { name: 'Luxemburg settings' })

      fireEvent.click(screen.getByRole('button', { name: 'Back' }))
      // Out to the right, still there, but hidden and inert, and the chapter is back in use.
      expect(sliding('page-out-right')).toContainElement(page)
      expect(sliding('page-out-right')!.parentElement).toHaveAttribute('inert')
      expect(screen.queryByRole('region', { name: 'Luxemburg settings' })).toBeNull()
      expect(chapterCard()).not.toHaveAttribute('inert')
      expect(chapterCard()).not.toHaveAttribute('aria-hidden')
      expect(screen.getByRole('heading', { level: 1 })).toHaveAccessibleName('Luxemburg')
      expect(pen).toHaveFocus()

      endAnimation(sliding('page-out-right')!)
      expect(page).not.toBeInTheDocument()
      expect(document.querySelector('[class*="page-"]')).toBeNull()
    })

    it('closes with Escape the same way', async () => {
      prism()
      vi.mocked(Bindings.GetRunLogs).mockResolvedValue([] as never)
      render(<App />)
      await screen.findByRole('heading', { level: 1 })
      fireEvent.click(screen.getByRole('button', { name: 'Logs for Luxemburg' }))
      await screen.findByRole('region', { name: 'Luxemburg logs' })
      expect(sliding('page-in-right')).not.toBeNull()
      fireEvent.keyDown(window, { key: 'Escape' })
      expect(sliding('page-out-right')).not.toBeNull()
      expect(screen.queryByRole('region', { name: 'Luxemburg logs' })).toBeNull()
      endAnimation(sliding('page-out-right')!)
      expect(sliding('page-out-right')).toBeNull()
    })

    it('slides the settings down from the top over the card area, and back up on close', async () => {
      render(<App />)
      await screen.findByRole('heading', { level: 1 })
      const gear = screen.getByRole('button', { name: 'Settings' })
      act(() => gear.focus())
      fireEvent.click(gear)
      const page = await screen.findByRole('region', { name: 'Settings' })
      const layer = page.parentElement!
      expect(layer).toHaveClass('page-in-top')
      // Over the whole card area: no rounded clip of its own, and the stage is its frame.
      expect(layer.parentElement).not.toHaveClass('overflow-hidden')
      expect(layer.parentElement!.parentElement).toBe(chapterCard().parentElement)
      expect(chapterCard()).toHaveAttribute('inert')
      await waitFor(() => expect(layer).toHaveFocus())

      // The gear toggles it.
      fireEvent.click(gear)
      expect(sliding('page-out-top')).toContainElement(page)
      expect(chapterCard()).not.toHaveAttribute('inert')
      endAnimation(sliding('page-out-top')!)
      expect(page).not.toBeInTheDocument()
      expect(gear).toHaveFocus()
    })

    it('opens the account from the sidebar tile, over the settings, and closes it from the tile (issue 192)', async () => {
      render(<App />)
      await screen.findByRole('heading', { level: 1 })
      fireEvent.click(screen.getByRole('button', { name: 'Settings' }))
      await screen.findByRole('region', { name: 'Settings' })

      const tile = screen.getByRole('button', { name: /^Account, / })
      expect(tile).toHaveAttribute('aria-pressed', 'false')
      fireEvent.click(tile)
      const account = await screen.findByRole('region', { name: 'Account' }, { timeout: 5000 })
      expect(screen.queryByRole('region', { name: 'Settings' })).toBeNull()
      expect(tile).toHaveAttribute('aria-pressed', 'true')

      fireEvent.click(tile)
      expect(sliding('page-out-top')).toContainElement(account)
      endAnimation(sliding('page-out-top')!)
      expect(account).not.toBeInTheDocument()
      expect(tile).toHaveAttribute('aria-pressed', 'false')
    })

    it("closes a chapter's page when another chapter is picked, sliding it out as the card changes", async () => {
      prism()
      render(<App />)
      await screen.findByRole('heading', { level: 1 })
      fireEvent.click(screen.getByRole('button', { name: 'Luxemburg settings' }))
      await screen.findByRole('region', { name: 'Luxemburg settings' })

      fireEvent.click(screen.getByRole('button', { name: /03.*Frangfurd/ }))
      expect(sliding('page-out-right')).not.toBeNull()
      expect(screen.queryByRole('region', { name: 'Luxemburg settings' })).toBeNull()
      endAnimation(sliding('page-out-right')!)
      expect(sliding('page-out-right')).toBeNull()
      expect(screen.getByRole('button', { name: 'Logs for Frangfurd' })).toBeInTheDocument()
    })

    it('closes the settings when a chapter is picked, and a chapter page when the settings open', async () => {
      prism()
      render(<App />)
      await screen.findByRole('heading', { level: 1 })
      fireEvent.click(screen.getByRole('button', { name: 'Luxemburg settings' }))
      await screen.findByRole('region', { name: 'Luxemburg settings' })

      fireEvent.click(screen.getByRole('button', { name: 'Settings' }))
      await screen.findByRole('region', { name: 'Settings' })
      // The chapter's page leaves as the settings arrive, never both open.
      expect(sliding('page-out-right')).not.toBeNull()
      expect(sliding('page-in-top')).not.toBeNull()
      expect(screen.queryByRole('region', { name: 'Luxemburg settings' })).toBeNull()

      fireEvent.click(screen.getByRole('button', { name: /02.*Lichdenstein/ }))
      expect(sliding('page-out-top')).not.toBeNull()
      expect(screen.queryByRole('region', { name: 'Settings' })).toBeNull()
    })

    it('slides the frame in at once and reveals the body only when the data is there', async () => {
      render(<App />)
      await screen.findByRole('heading', { level: 1 })
      // The settings load on mount; hold them back to see the page wait.
      act(() => useSettingsStore.setState({ loaded: false }))
      fireEvent.click(screen.getByRole('button', { name: 'Settings' }))
      const page = await screen.findByRole('region', { name: 'Settings' })
      expect(sliding('page-in-top')).toContainElement(page)
      expect(within(page).getByRole('heading', { level: 1 })).toHaveTextContent('Settings')
      expect(within(page).getByText('Reading.')).toBeInTheDocument()
      expect(within(page).queryByLabelText('Prism program')).toBeNull()
      act(() => useSettingsStore.setState({ loaded: true }))
      expect(within(page).getByLabelText('Prism program')).toBeInTheDocument()
    })
  })
})
