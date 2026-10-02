import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as App from '../../wailsjs/go/main/App'
import { models } from '../../wailsjs/go/models'
import { selectInstalled, selectInstancePack, useEngineStore } from './useEngineStore'
import type { EngineInfo } from '../types'

vi.mock('../../wailsjs/go/main/App')

const found: EngineInfo = {
  found: true,
  executable: 'C:/Prism/prismlauncher.exe',
  version: '11.1.0',
  root: '',
  source: 'path',
}

describe('useEngineStore', () => {
  beforeEach(() => {
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
    vi.mocked(App.GetPrismRelease).mockReset()
    vi.mocked(App.InstallPrism).mockReset()
    vi.mocked(App.GetEngine).mockReset()
    vi.mocked(App.GetInstances).mockReset()
    vi.mocked(App.RefreshEngine).mockReset()
    vi.mocked(App.LaunchChapter).mockReset()
    vi.mocked(App.InstallChapter).mockReset()
    vi.mocked(App.SetPackSource).mockReset()
    vi.mocked(App.GetPackStates).mockReset()
  })

  it('installs a chapter and takes the instances Go read back', async () => {
    const report = { root: 'C:/Prism', dir: 'C:/Prism/instances', present: { frangfurd: true } }
    vi.mocked(App.InstallChapter).mockResolvedValue(models.InstanceReport.createFrom(report))
    const pending = useEngineStore.getState().installChapter('frangfurd')
    expect(useEngineStore.getState().installing).toBe('frangfurd')
    await pending
    const s = useEngineStore.getState()
    expect(s.installing).toBeNull()
    expect(s.installedNow).toBe('frangfurd')
    expect(selectInstalled('frangfurd')(s)).toBe(true)
  })

  it('records a failed install and leaves the instances as they were', async () => {
    const before = models.InstanceReport.createFrom({
      root: 'C:/Prism',
      dir: 'C:/Prism/instances',
      present: { frangfurd: false },
    })
    useEngineStore.setState({ instances: before })
    vi.mocked(App.InstallChapter).mockRejectedValue('frangfurd has no hosted pack to install')
    await useEngineStore.getState().installChapter('frangfurd')
    const s = useEngineStore.getState()
    expect(s.error).toBe('frangfurd has no hosted pack to install')
    expect(s.installing).toBeNull()
    expect(s.installedNow).toBeNull()
    expect(s.instances).toBe(before)
  })

  it('switches the pack source, takes the report Go returns and checks the pack again', async () => {
    const local = 'http://localhost:8080/pack.toml'
    const report = {
      root: 'C:/Prism',
      dir: 'C:/Prism/instances',
      present: { frangfurd: true },
      packUrl: { frangfurd: local },
    }
    useEngineStore.setState({
      packStates: {
        frangfurd: {
          chapterId: 'frangfurd',
          installed: true,
          checked: true,
          upToDate: true,
          version: '1',
        },
      },
    })
    vi.mocked(App.SetPackSource).mockResolvedValue(models.InstanceReport.createFrom(report))
    vi.mocked(App.GetPackStates).mockResolvedValue([
      models.PackState.createFrom({
        chapterId: 'frangfurd',
        installed: true,
        checked: true,
        upToDate: false,
        version: '2',
      }),
    ])
    await useEngineStore.getState().setPackSource('frangfurd', 'dev')
    expect(App.SetPackSource).toHaveBeenCalledWith('frangfurd', 'dev')
    const s = useEngineStore.getState()
    expect(selectInstancePack('frangfurd')(s)).toBe(local)
    expect(s.packStates.frangfurd?.upToDate).toBe(false)
  })

  it('rethrows a refused switch and leaves the instances and pack states as they were', async () => {
    const before = models.InstanceReport.createFrom({
      root: 'C:/Prism',
      dir: 'C:/Prism/instances',
      present: { frangfurd: true },
    })
    useEngineStore.setState({ instances: before })
    vi.mocked(App.SetPackSource).mockRejectedValue(
      'Frangfurd is starting or running; close the game first',
    )
    await expect(useEngineStore.getState().setPackSource('frangfurd', 'published')).rejects.toBe(
      'Frangfurd is starting or running; close the game first',
    )
    expect(useEngineStore.getState().instances).toBe(before)
    expect(App.GetPackStates).not.toHaveBeenCalled()
  })

  it('reads the engine and degrades to unknown without a bridge', async () => {
    vi.mocked(App.GetEngine).mockResolvedValue(found)
    await useEngineStore.getState().load()
    expect(useEngineStore.getState().engine).toEqual(found)

    vi.mocked(App.GetEngine).mockImplementation(() => {
      throw new TypeError('no bridge')
    })
    await useEngineStore.getState().load()
    expect(useEngineStore.getState().engine).toBeNull()
  })

  it('reads instances with the engine and after a refresh, unknown without a bridge', async () => {
    const report = { root: 'C:/Prism', dir: 'C:/Prism/instances', present: { luxemburg: true } }
    vi.mocked(App.GetEngine).mockResolvedValue(found)
    vi.mocked(App.GetInstances).mockResolvedValue(models.InstanceReport.createFrom(report))
    await useEngineStore.getState().load()
    expect(selectInstalled('luxemburg')(useEngineStore.getState())).toBe(true)
    expect(selectInstalled('frangfurd')(useEngineStore.getState())).toBeUndefined()

    vi.mocked(App.RefreshEngine).mockResolvedValue(found)
    vi.mocked(App.GetInstances).mockResolvedValue(
      models.InstanceReport.createFrom({ ...report, present: { luxemburg: false } }),
    )
    await useEngineStore.getState().refresh()
    expect(selectInstalled('luxemburg')(useEngineStore.getState())).toBe(false)

    vi.mocked(App.GetInstances).mockImplementation(() => {
      throw new TypeError('no bridge')
    })
    await useEngineStore.getState().loadInstances()
    expect(useEngineStore.getState().instances).toBeNull()
  })

  it('records a refresh failure', async () => {
    vi.mocked(App.RefreshEngine).mockRejectedValue('read settings: boom')
    await useEngineStore.getState().refresh()
    expect(useEngineStore.getState().error).toBe('read settings: boom')
  })

  it('tracks the launching chapter and surfaces a launch error', async () => {
    let resolve!: () => void
    vi.mocked(App.LaunchChapter).mockReturnValue(new Promise<void>((r) => (resolve = r)))
    const p = useEngineStore.getState().launch('luxemburg')
    expect(useEngineStore.getState().launching).toBe('luxemburg')
    resolve()
    await p
    expect(useEngineStore.getState().launching).toBeNull()
    expect(useEngineStore.getState().error).toBeNull()

    vi.mocked(App.LaunchChapter).mockRejectedValue(new Error('Prism Launcher was not found'))
    await useEngineStore.getState().launch('frangfurd')
    expect(useEngineStore.getState().error).toContain('not found')
    useEngineStore.getState().clearError()
    expect(useEngineStore.getState().error).toBeNull()
  })

  it('installs Prism: progress from events, outcome from the promise, then re-reads', async () => {
    const release = {
      version: '11.1.1',
      asset: 'PrismLauncher-Windows-MSVC-Portable-11.1.1.zip',
      url: 'https://github.com/x',
      size: 20396629,
      digest: 'sha256:ab',
      page: 'https://github.com/p',
      installed: '',
      updateAvailable: false,
    }
    vi.mocked(App.GetPrismRelease).mockResolvedValue(models.PrismRelease.createFrom(release))
    await useEngineStore.getState().loadRelease()
    expect(useEngineStore.getState().release?.version).toBe('11.1.1')

    // Before an install nothing is shown, whatever arrives.
    useEngineStore
      .getState()
      .receiveInstall({ phase: 'downloading', received: 1, total: 2, error: '' })
    expect(useEngineStore.getState().install).toBeNull()

    let finish!: () => void
    vi.mocked(App.InstallPrism).mockReturnValue(new Promise<void>((r) => (finish = r)))
    vi.mocked(App.GetEngine).mockResolvedValue({ ...found, source: 'managed' })
    const p = useEngineStore.getState().installPrism()
    expect(useEngineStore.getState().install?.phase).toBe('downloading')
    useEngineStore
      .getState()
      .receiveInstall({ phase: 'verifying', received: 0, total: 0, error: '' })
    expect(useEngineStore.getState().install?.phase).toBe('verifying')
    finish()
    await p
    expect(useEngineStore.getState().install?.phase).toBe('done')
    expect(useEngineStore.getState().engine?.source).toBe('managed')
    expect(App.GetPrismRelease).toHaveBeenCalledTimes(2)

    // A late event does not reopen a finished install.
    useEngineStore
      .getState()
      .receiveInstall({ phase: 'downloading', received: 1, total: 2, error: '' })
    expect(useEngineStore.getState().install?.phase).toBe('done')

    // The first Play clears "ready, sign in on first play".
    vi.mocked(App.LaunchChapter).mockResolvedValue()
    await useEngineStore.getState().launch('frangfurd')
    expect(useEngineStore.getState().install).toBeNull()
  })

  it('records a failed install, and offers nothing without a bridge', async () => {
    vi.mocked(App.InstallPrism).mockRejectedValue("Prism's download does not match GitHub's digest")
    await useEngineStore.getState().installPrism()
    expect(useEngineStore.getState().install).toMatchObject({ phase: 'failed' })
    expect(useEngineStore.getState().install?.error).toMatch(/digest/)

    vi.mocked(App.GetPrismRelease).mockImplementation(() => {
      throw new TypeError('no bridge')
    })
    await useEngineStore.getState().loadRelease()
    expect(useEngineStore.getState().release).toBeNull()
    expect(useEngineStore.getState().listenInstall()).toBeTypeOf('function')
  })
})
