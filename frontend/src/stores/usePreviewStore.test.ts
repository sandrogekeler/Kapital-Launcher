import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as App from '../../wailsjs/go/main/App'
import { usePreviewStore } from './usePreviewStore'
import { useEngineStore } from './useEngineStore'
import { useGameStore } from './useGameStore'
import type { EngineInfo, GameState, PreviewSituation, PrismRelease } from '../types'

vi.mock('../../wailsjs/go/main/App')
vi.mock('../../wailsjs/runtime/runtime')

const attachBridge = () => Object.assign(window, { go: {} })
const detachBridge = () => {
  delete (window as unknown as { go?: unknown }).go
}

const crashed: PreviewSituation = {
  id: 'crashed',
  label: 'Crashed, with a crash report',
  scope: 'chapter',
  card: true,
  playsInstall: false,
}
const missing: PreviewSituation = {
  id: 'prism-missing',
  label: 'Prism missing',
  scope: 'prism',
  card: false,
  playsInstall: false,
}
const install: PreviewSituation = { ...missing, id: 'prism-install', playsInstall: true }

const none: EngineInfo = { found: false, executable: '', version: '', root: '', source: '' }
const managed: EngineInfo = { ...none, found: true, version: '9.0.0', source: 'managed' }
const linked: EngineInfo = { ...none, found: true, version: '8.0', source: 'path' }
const release: PrismRelease = {
  version: '9.9.9',
  asset: 'a.zip',
  url: 'https://github.com/x',
  size: 20,
  digest: 'sha256:0',
  page: 'https://github.com/x/releases',
  installed: '',
  updateAvailable: false,
}

describe('usePreviewStore', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    attachBridge()
    usePreviewStore.setState({ situations: [] })
    useEngineStore.setState({ engine: null, release: null, install: null })
    useGameStore.setState({ states: {}, stopErrors: {} })
    vi.mocked(App.GetEngine).mockResolvedValue(none)
    vi.mocked(App.GetInstances).mockResolvedValue({
      root: '',
      dir: '',
      present: {},
      packUrl: {},
      sizeBytes: {},
    })
    vi.mocked(App.GetPackStates).mockResolvedValue([])
    vi.mocked(App.GetGameStates).mockResolvedValue([])
    vi.mocked(App.StartPreview).mockResolvedValue({ cardSkipped: false })
    vi.mocked(App.ClearPreviews).mockResolvedValue()
  })
  afterEach(detachBridge)

  it("holds Go's list of situations, and none when Go cannot say", async () => {
    vi.mocked(App.GetPreviewSituations).mockResolvedValue([crashed, missing] as never)
    await usePreviewStore.getState().load()
    expect(usePreviewStore.getState().situations).toEqual([crashed, missing])

    vi.mocked(App.GetPreviewSituations).mockRejectedValue('no backend')
    await usePreviewStore.getState().load()
    expect(usePreviewStore.getState().situations).toEqual([])
  })

  it('asks Go for the chapter and the situation, then reads the screens again', async () => {
    useEngineStore.setState({ release })
    await usePreviewStore.getState().start('frangfurd', crashed)
    expect(App.StartPreview).toHaveBeenCalledExactlyOnceWith('frangfurd', 'crashed')
    expect(App.GetEngine).toHaveBeenCalled()
    expect(App.GetInstances).toHaveBeenCalled()
    expect(App.GetPackStates).toHaveBeenCalled()
    // No Prism, so the release is read again too, and a made-up one is not kept.
    expect(App.GetPrismRelease).toHaveBeenCalled()
    expect(App.InstallPrism).not.toHaveBeenCalled()
  })

  it('does not keep a made-up release for a Prism that cannot be updated', async () => {
    vi.mocked(App.GetEngine).mockResolvedValue(linked)
    useEngineStore.setState({ release: { ...release, updateAvailable: true } })
    await usePreviewStore.getState().clear()
    expect(useEngineStore.getState().release).toBeNull()
    expect(App.GetPrismRelease).not.toHaveBeenCalled()

    // The launcher's own copy is the one it offers updates for.
    vi.mocked(App.GetEngine).mockResolvedValue(managed)
    vi.mocked(App.GetPrismRelease).mockResolvedValue(release)
    await usePreviewStore.getState().clear()
    expect(useEngineStore.getState().release).toEqual(release)
  })

  it('plays the made-up install for the situation that is one, and shows it fail', async () => {
    vi.mocked(App.GetPrismRelease).mockResolvedValue(release)
    vi.mocked(App.InstallPrism).mockRejectedValue('The download failed (preview)')
    await usePreviewStore.getState().start('frangfurd', install)
    await vi.waitFor(() => expect(useEngineStore.getState().install?.phase).toBe('failed'))
    expect(useEngineStore.getState().install?.error).toBe('The download failed (preview)')
    expect(App.InstallPrism).toHaveBeenCalledOnce()
  })

  it('forgets a finished install, which is not news after a preview', async () => {
    useEngineStore.setState({
      install: { phase: 'failed', received: 0, total: 0, error: 'The download failed' },
    })
    await usePreviewStore.getState().start('frangfurd', missing)
    expect(useEngineStore.getState().install).toBeNull()
  })

  it('rethrows a refusal for the section to show', async () => {
    vi.mocked(App.StartPreview).mockRejectedValue('no preview "x"')
    await expect(usePreviewStore.getState().start('frangfurd', crashed)).rejects.toBe(
      'no preview "x"',
    )
    expect(App.GetEngine).not.toHaveBeenCalled()
  })

  it('has nothing to ask without the desktop app, and says so', async () => {
    detachBridge()
    await expect(usePreviewStore.getState().start('frangfurd', crashed)).rejects.toThrow(
      'desktop app',
    )
    await expect(usePreviewStore.getState().clear()).rejects.toThrow('desktop app')
    expect(App.StartPreview).not.toHaveBeenCalled()
    expect(App.ClearPreviews).not.toHaveBeenCalled()
  })

  it("clears, then reads the chapters' real states", async () => {
    const real: GameState = {
      chapterId: 'frangfurd',
      phase: 'idle',
      since: '',
      startedAt: '',
    }
    useGameStore.setState({ states: { frangfurd: { ...real, phase: 'crashed' } } })
    vi.mocked(App.GetGameStates).mockResolvedValue([real] as never)
    await usePreviewStore.getState().clear()
    expect(App.ClearPreviews).toHaveBeenCalledOnce()
    expect(useGameStore.getState().states.frangfurd?.phase).toBe('idle')
  })
})
