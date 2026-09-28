import { create } from 'zustand'
import type { Chapter, Manifest } from '../types'
import { BUNDLED_MANIFEST, chapterById } from '../lib/manifest'
import { readOr } from '../lib/ipc'
import { GetManifest } from '../../wailsjs/go/main/App'

/**
 * The chapter list and which chapter is open. One store, one domain: nothing
 * about the engine or the settings lives here.
 *
 * The bundled manifest is the initial state, so the first render already has
 * chapters; `load` replaces it with what Go holds, which is the same file today
 * and will be the site's copy once ADR-4 lands.
 */
interface ChapterStore {
  manifest: Manifest
  selectedId: string
  loaded: boolean
  load: () => Promise<void>
  select: (id: string) => void
}

const firstId = (m: Manifest) => m.chapters[0]?.id ?? ''

export const useChapterStore = create<ChapterStore>((set, get) => ({
  manifest: BUNDLED_MANIFEST,
  selectedId: firstId(BUNDLED_MANIFEST),
  loaded: false,

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

  select: (id) => {
    if (chapterById(get().manifest, id)) set({ selectedId: id })
  },
}))

/** The open chapter, always defined once the manifest has one entry. */
export const selectChapter = (s: ChapterStore): Chapter | undefined =>
  chapterById(s.manifest, s.selectedId)
