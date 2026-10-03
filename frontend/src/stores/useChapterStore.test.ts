import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as App from '../../wailsjs/go/main/App'
import { models } from '../../wailsjs/go/models'
import { selectChapter, selectWikiPick, useChapterStore } from './useChapterStore'
import { BUNDLED_MANIFEST } from '../lib/manifest'
import type { WikiPage, WikiShot } from '../types'

vi.mock('../../wailsjs/go/main/App')

describe('useChapterStore', () => {
  beforeEach(() => {
    useChapterStore.setState({
      manifest: BUNDLED_MANIFEST,
      selectedId: BUNDLED_MANIFEST.chapters[0]!.id,
      loaded: false,
      wikiPages: [],
      wikiShots: [],
      slides: {},
    })
    vi.mocked(App.GetManifest).mockReset()
    vi.mocked(App.GetWikiPages).mockReset()
    vi.mocked(App.GetWikiShots).mockReset()
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

  it('picks a wiki page for the open chapter when the pages arrive, and again on each switch', async () => {
    const page = (url: string, era: string): WikiPage => ({
      title: url,
      line: 'l',
      url,
      eras: [era],
      id: url,
      related: [],
    })
    vi.mocked(App.GetWikiPages).mockResolvedValue([
      page('/a', 'Luxemburg'),
      page('/b', 'Frangfurd'),
      page('/c', 'Frangfurd'),
    ])
    await useChapterStore.getState().loadWikiPages()
    expect(selectWikiPick(useChapterStore.getState())?.url).toBe('/a')

    useChapterStore.getState().select('frangfurd')
    const first = selectWikiPick(useChapterStore.getState())?.url
    expect(['/b', '/c']).toContain(first)
    // Leaving and coming back never shows the same page twice in a row.
    useChapterStore.getState().select('luxemburg')
    useChapterStore.getState().select('frangfurd')
    expect(selectWikiPick(useChapterStore.getState())?.url).not.toBe(first)
    // Clicking the open chapter again is not a switch: the page stays (#140).
    const shown = selectWikiPick(useChapterStore.getState())?.url
    useChapterStore.getState().select('frangfurd')
    useChapterStore.getState().select('frangfurd')
    expect(selectWikiPick(useChapterStore.getState())?.url).toBe(shown)
    // A chapter without pages keeps the teaser.
    useChapterStore.getState().select('lichdenstein')
    expect(selectWikiPick(useChapterStore.getState())).toBeUndefined()
  })

  it('has no pick without a bridge or a reachable wiki', async () => {
    vi.mocked(App.GetWikiPages).mockRejectedValue('wiki pages: offline (no cached copy)')
    await useChapterStore.getState().loadWikiPages()
    expect(useChapterStore.getState().wikiPages).toEqual([])
    expect(selectWikiPick(useChapterStore.getState())).toBeUndefined()
  })

  describe('slides (issue 142)', () => {
    const page = (id: string, era: string, related: string[] = []): WikiPage => ({
      title: id,
      line: 'l',
      url: `/wiki/${id}`,
      eras: [era],
      id,
      related,
    })
    const shot = (src: string, era: string, subject = ''): WikiShot => ({ era, src, subject })

    it('gives the open chapter a picture once the screenshots arrive, with a post about it', async () => {
      vi.mocked(App.GetWikiPages).mockResolvedValue([
        page('castle', 'Luxemburg', ['avari']),
        page('avari', 'Luxemburg', ['castle']),
        page('okinowa', 'Luxemburg'),
      ])
      vi.mocked(App.GetWikiShots).mockResolvedValue([
        shot('/wiki-art/luxemburg/1.webp', 'Luxemburg', 'castle'),
      ])
      await useChapterStore.getState().loadWikiPages()
      expect(useChapterStore.getState().slides.luxemburg?.art).toBeUndefined()
      await useChapterStore.getState().loadWikiShots()
      const slide = useChapterStore.getState().slides.luxemburg!
      expect(slide.art).toBe('/wiki-art/luxemburg/1.webp')
      expect(['castle', 'avari']).toContain(slide.page?.id)
    })

    it('moves on to another picture and post on advance, and keeps them on a click of the open chapter', async () => {
      useChapterStore.setState({
        wikiPages: [page('castle', 'Luxemburg', ['avari']), page('avari', 'Luxemburg', ['castle'])],
        wikiShots: [shot('/1', 'Luxemburg', 'castle'), shot('/2', 'Luxemburg', 'castle')],
      })
      await useChapterStore.getState().advance()
      const first = useChapterStore.getState().slides.luxemburg!
      await useChapterStore.getState().advance()
      const next = useChapterStore.getState().slides.luxemburg!
      expect(next.art).not.toBe(first.art)
      expect(next.page?.id).not.toBe(first.page?.id)
      useChapterStore.getState().select('luxemburg')
      expect(useChapterStore.getState().slides.luxemburg).toBe(next)
    })

    it('has no screenshots without a bridge, and the bundled art stays', async () => {
      vi.mocked(App.GetWikiShots).mockRejectedValue('wiki pages: offline (no cached copy)')
      await useChapterStore.getState().loadWikiShots()
      expect(useChapterStore.getState().wikiShots).toEqual([])
      expect(useChapterStore.getState().slides.luxemburg).toBeUndefined()
    })
  })
})
