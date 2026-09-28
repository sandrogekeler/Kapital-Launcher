import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as App from '../../wailsjs/go/main/App'
import { models } from '../../wailsjs/go/models'
import { selectChapter, useChapterStore } from './useChapterStore'
import { BUNDLED_MANIFEST } from '../lib/manifest'

vi.mock('../../wailsjs/go/main/App')

describe('useChapterStore', () => {
  beforeEach(() => {
    useChapterStore.setState({
      manifest: BUNDLED_MANIFEST,
      selectedId: BUNDLED_MANIFEST.chapters[0]!.id,
      loaded: false,
    })
    vi.mocked(App.GetManifest).mockReset()
  })

  it('starts on the bundled manifest with the first chapter open', () => {
    const s = useChapterStore.getState()
    expect(s.manifest.chapters.length).toBe(3)
    expect(selectChapter(s)?.id).toBe('luxemburg')
  })

  it('keeps the bundled manifest when the bridge is missing', async () => {
    vi.mocked(App.GetManifest).mockImplementation(() => {
      throw new TypeError('window.go is undefined')
    })
    await useChapterStore.getState().load()
    expect(useChapterStore.getState().manifest).toBe(BUNDLED_MANIFEST)
    expect(useChapterStore.getState().loaded).toBe(true)
  })

  it('takes the backend manifest and keeps a still-valid selection', async () => {
    const trimmed = { ...BUNDLED_MANIFEST, chapters: BUNDLED_MANIFEST.chapters.slice(1) }
    useChapterStore.getState().select('frangfurd')
    vi.mocked(App.GetManifest).mockResolvedValue(models.Manifest.createFrom(trimmed))
    await useChapterStore.getState().load()
    expect(useChapterStore.getState().selectedId).toBe('frangfurd')

    useChapterStore.getState().select('lichdenstein')
    vi.mocked(App.GetManifest).mockResolvedValue(
      models.Manifest.createFrom({
        ...BUNDLED_MANIFEST,
        chapters: [BUNDLED_MANIFEST.chapters[2]!],
      }),
    )
    await useChapterStore.getState().load()
    expect(useChapterStore.getState().selectedId).toBe('frangfurd')
  })

  it('ignores a selection the manifest does not have', () => {
    useChapterStore.getState().select('atlantis')
    expect(useChapterStore.getState().selectedId).toBe('luxemburg')
  })
})
