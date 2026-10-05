import { create } from 'zustand'
import type { Chapter, Manifest, Panorama, WikiPage, WikiShot } from '../types'
import { BUNDLED_MANIFEST, chapterById } from '../lib/manifest'
import { pickSlide, preload, shotsForChapter, type Slide } from '../lib/slides'
import { readOr } from '../lib/ipc'
import { GetManifest, GetPanoramas, GetWikiPages, GetWikiShots } from '../../wailsjs/go/main/App'

/**
 * The chapter list, which chapter is open, and the wiki page shown for it.
 * One store, one domain: nothing about the engine or the settings lives here.
 *
 * The bundled manifest is the initial state, so the first render already has
 * chapters; `load` replaces it with what Go holds, which is the same file today
 * and will be the site's copy once ADR-4 lands.
 *
 * The wiki's pages come from its lore export through Go (#58), and its
 * screenshots with them, downloaded and served by Go (#141). Each chapter has a
 * slide, a picture of its era and a post linked to what it shows (#142, see
 * lib/slides.ts), picked again on every switch to it and on `advance`, which
 * the rotation calls every minute. With no screenshots the hero keeps the
 * bundled art; with no pages (offline on a first start) the panel shows the
 * manifest's teaser.
 *
 * A chapter's panorama (issue 195) is six faces Go cached from the resources pack
 * in its instance, by chapter id; a chapter without one is not in the record and
 * its hero shows its own picture. It is read when the panorama is chosen and
 * again when the window is focused or an install or a run ends, which is the
 * caller's to ask (`loadPanoramas`), never a timer.
 */
interface ChapterStore {
  manifest: Manifest
  selectedId: string
  loaded: boolean
  /** The wiki's pages, empty until loaded or when they could not be read. */
  wikiPages: WikiPage[]
  /** The wiki's screenshots, empty until loaded or when there are none. */
  wikiShots: WikiShot[]
  /** Each chapter's slide, by chapter id; picked again on every switch to it. */
  slides: Record<string, Slide | undefined>
  /** The six faces of each chapter's panorama, by chapter id; a chapter with none is absent. */
  panoramas: Record<string, string[] | undefined>
  load: () => Promise<void>
  loadWikiPages: () => Promise<void>
  loadWikiShots: () => Promise<void>
  loadPanoramas: () => Promise<void>
  select: (id: string) => void
  /** The open chapter's next slide, shown once its picture has loaded. */
  advance: () => Promise<void>
}

/** Go encodes an empty list as null; anything that is not a list is none. */
const asList = <T>(raw: unknown): T[] => (Array.isArray(raw) ? (raw as T[]) : [])

const firstId = (m: Manifest) => m.chapters[0]?.id ?? ''

export const useChapterStore = create<ChapterStore>((set, get) => ({
  manifest: BUNDLED_MANIFEST,
  selectedId: firstId(BUNDLED_MANIFEST),
  loaded: false,
  wikiPages: [],
  wikiShots: [],
  slides: {},
  panoramas: {},

  load: async () => {
    const manifest = await readOr(GetManifest, get().manifest)
    const { selectedId } = get()
    set({
      manifest,
      loaded: true,
      // Keep the selection if the new list still has it; otherwise fall back
      // to the first chapter rather than pointing at nothing.
      selectedId: chapterById(manifest, selectedId) ? selectedId : firstId(manifest),
    })
  },

  // A read: with no bridge, or with neither the wiki nor a cache reachable,
  // there are no pages and the teaser stands. The open chapter gets its first
  // slide as soon as the pages arrive.
  loadWikiPages: async () => {
    const wikiPages = asList<WikiPage>(await readOr(GetWikiPages, []))
    set({ wikiPages })
    const { manifest, selectedId, wikiShots, slides } = get()
    const chapter = chapterById(manifest, selectedId)
    if (chapter)
      set({
        slides: { ...slides, [selectedId]: pickSlide(wikiShots, wikiPages, chapter, undefined) },
      })
  },

  // Go waits for the downloads, so the pictures come after the pages. The open
  // chapter's slide takes a picture as soon as there is one for it. Read again
  // when the number of pictures changes (issue 172): a new draw prunes what it
  // did not pick, so the open slide, if its picture is gone, takes another. The
  // other chapters' slides are picked again when they are switched to.
  loadWikiShots: async () => {
    const wikiShots = asList<WikiShot>(await readOr(GetWikiShots, []))
    set({ wikiShots })
    const { manifest, selectedId, wikiPages, slides } = get()
    const chapter = chapterById(manifest, selectedId)
    const art = slides[selectedId]?.art
    if (
      !chapter ||
      wikiShots.some((s) => s.src === art) ||
      shotsForChapter(wikiShots, chapter).length === 0
    )
      return
    const next = pickSlide(wikiShots, wikiPages, chapter, slides[selectedId])
    await preload(next.art)
    if (get().selectedId === selectedId) set({ slides: { ...get().slides, [selectedId]: next } })
  },

  // A read of what Go found in the instances' resources packs. Without a bridge,
  // or on a failure, there is none and each hero keeps its own picture.
  loadPanoramas: async () => {
    const found = asList<Panorama>(await readOr(GetPanoramas, []))
    const panoramas: Record<string, string[]> = {}
    for (const p of found) {
      if (Array.isArray(p.faces) && p.faces.length === 6) panoramas[p.chapterId] = p.faces
    }
    set({ panoramas })
  },

  select: (id) => {
    // Clicking the open chapter is not a switch: its slide stays (#140).
    if (id === get().selectedId) return
    const chapter = chapterById(get().manifest, id)
    if (!chapter) return
    const { wikiPages, wikiShots, slides } = get()
    set({
      selectedId: id,
      slides: { ...slides, [id]: pickSlide(wikiShots, wikiPages, chapter, slides[id]) },
    })
  },

  advance: async () => {
    const { manifest, selectedId, wikiPages, wikiShots, slides } = get()
    const chapter = chapterById(manifest, selectedId)
    if (!chapter) return
    const next = pickSlide(wikiShots, wikiPages, chapter, slides[selectedId])
    await preload(next.art)
    // A switch while the picture loaded has picked that chapter's own slide.
    if (get().selectedId === selectedId) set({ slides: { ...get().slides, [selectedId]: next } })
  },
}))

/** The open chapter, always defined once the manifest has one entry. */
export const selectChapter = (s: ChapterStore): Chapter | undefined =>
  chapterById(s.manifest, s.selectedId)

/** The wiki page picked for the open chapter, or undefined for the manifest's teaser. */
export const selectWikiPick = (s: ChapterStore): WikiPage | undefined =>
  s.slides[s.selectedId]?.page

/** A chapter's slide, undefined until the wiki has answered. */
export const selectSlide =
  (id: string) =>
  (s: ChapterStore): Slide | undefined =>
    s.slides[id]

/** A chapter's panorama faces, undefined where it has none or has not been read. */
export const selectPanorama =
  (id: string) =>
  (s: ChapterStore): string[] | undefined =>
    s.panoramas[id]

/** Whether the open chapter has more than one slide to rotate through. */
export const selectCanRotate = (s: ChapterStore): boolean => {
  const chapter = chapterById(s.manifest, s.selectedId)
  return !!chapter && (shotsForChapter(s.wikiShots, chapter).length > 1 || s.wikiPages.length > 1)
}
