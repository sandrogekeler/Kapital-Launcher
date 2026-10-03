import { create } from 'zustand'
import type { Chapter, Manifest, WikiPage } from '../types'
import { BUNDLED_MANIFEST, chapterById } from '../lib/manifest'
import { pagesForChapter, pickPage } from '../lib/wiki'
import { readOr } from '../lib/ipc'
import { GetManifest, GetWikiPages } from '../../wailsjs/go/main/App'

/**
 * The chapter list, which chapter is open, and the wiki page shown for it.
 * One store, one domain: nothing about the engine or the settings lives here.
 *
 * The bundled manifest is the initial state, so the first render already has
 * chapters; `load` replaces it with what Go holds, which is the same file today
 * and will be the site's copy once ADR-4 lands.
 *
 * The wiki's pages come from its lore export through Go (#58). Every switch
 * picks a random page of the chapter's era; with no pages (offline on a first
 * start) the panel shows the manifest's teaser instead.
 */
interface ChapterStore {
  manifest: Manifest
  selectedId: string
  loaded: boolean
  /** The wiki's pages, empty until loaded or when they could not be read. */
  wikiPages: WikiPage[]
  /** The page picked for each chapter, by chapter id; re-picked on every switch to it. */
  wikiPick: Record<string, WikiPage | undefined>
  load: () => Promise<void>
  loadWikiPages: () => Promise<void>
  select: (id: string) => void
}

const firstId = (m: Manifest) => m.chapters[0]?.id ?? ''

export const useChapterStore = create<ChapterStore>((set, get) => ({
  manifest: BUNDLED_MANIFEST,
  selectedId: firstId(BUNDLED_MANIFEST),
  loaded: false,
  wikiPages: [],
  wikiPick: {},

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
  // pick as soon as the pages arrive.
  loadWikiPages: async () => {
    // Go encodes an empty list as null; anything that is not a list is none.
    const raw: unknown = await readOr(GetWikiPages, [])
    const wikiPages = Array.isArray(raw) ? (raw as WikiPage[]) : []
    set({ wikiPages })
    const { manifest, selectedId } = get()
    const chapter = chapterById(manifest, selectedId)
    if (chapter)
      set({ wikiPick: { [selectedId]: pickPage(pagesForChapter(wikiPages, chapter), undefined) } })
  },

  select: (id) => {
    // Clicking the open chapter is not a switch: its page stays (#140).
    if (id === get().selectedId) return
    const chapter = chapterById(get().manifest, id)
    if (!chapter) return
    const { wikiPages, wikiPick } = get()
    set({
      selectedId: id,
      wikiPick: { ...wikiPick, [id]: pickPage(pagesForChapter(wikiPages, chapter), wikiPick[id]) },
    })
  },
}))

/** The open chapter, always defined once the manifest has one entry. */
export const selectChapter = (s: ChapterStore): Chapter | undefined =>
  chapterById(s.manifest, s.selectedId)

/** The wiki page picked for the open chapter, or undefined for the manifest's teaser. */
export const selectWikiPick = (s: ChapterStore): WikiPage | undefined => s.wikiPick[s.selectedId]
