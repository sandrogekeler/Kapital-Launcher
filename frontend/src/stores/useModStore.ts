import { create } from 'zustand'
import type { ChapterMods } from '../types'
import { errMsg, hasWailsBridge } from '../lib/ipc'
import { withJars, withJarsSet } from '../lib/mods'
import { GetChapterMods, SetModsDisabled } from '../../wailsjs/go/main/App'

/**
 * A chapter's mods as the settings page shows them (issue 156): the jars in the instance's
 * mods folder with which are switched off, and the manifest's quick toggles resolved against
 * them. Go owns the list and the folder; this is the last answer, read when the section
 * opens, when the game ends and when the window regains focus. Only the lazy section
 * imports it, so none of it is in the launcher's first paint.
 *
 * A write shows at once and is replaced by Go's own answer; when a real backend refuses it
 * the view goes back to what it was and the refusal is rethrown for the section to show. With
 * no bridge (the browser-only preview) nothing can be read, and a write stands as drawn.
 */
interface ModStore {
  /** The last answer per chapter, absent before the first read. */
  byChapter: Record<string, ChapterMods | undefined>
  /** True when there is no Wails bridge, so there is nothing to list. */
  unavailable: boolean
  /** The reason a read was refused, by chapter. */
  readError: Record<string, string | undefined>
  /** The chapter whose write is in flight. */
  writing: string | null
  load: (chapterId: string) => Promise<void>
  /** Switches `jars` off or on in the chapter's folder and settings; rejects with Go's reason. */
  setJars: (chapterId: string, jars: readonly string[], off: boolean) => Promise<void>
}

export const useModStore = create<ModStore>((set, get) => ({
  byChapter: {},
  unavailable: false,
  readError: {},
  writing: null,

  load: async (chapterId) => {
    if (!hasWailsBridge()) {
      set({ unavailable: true })
      return
    }
    try {
      const mods = (await GetChapterMods(chapterId)) as ChapterMods
      set((s) => ({
        unavailable: false,
        byChapter: { ...s.byChapter, [chapterId]: mods },
        readError: { ...s.readError, [chapterId]: undefined },
      }))
    } catch (e) {
      set((s) => ({ readError: { ...s.readError, [chapterId]: errMsg(e) } }))
    }
  },

  setJars: async (chapterId, jars, off) => {
    const before = get().byChapter[chapterId]
    if (!before) return
    set((s) => ({
      writing: chapterId,
      byChapter: { ...s.byChapter, [chapterId]: withJarsSet(before, jars, off) },
    }))
    if (!hasWailsBridge()) {
      set({ writing: null })
      return
    }
    try {
      const saved = (await SetModsDisabled(
        chapterId,
        withJars(before.mods, jars, off),
      )) as ChapterMods
      set((s) => ({ writing: null, byChapter: { ...s.byChapter, [chapterId]: saved } }))
    } catch (e) {
      set((s) => ({ writing: null, byChapter: { ...s.byChapter, [chapterId]: before } }))
      // A rename that failed halfway leaves the list saved and the folder partly changed:
      // read again so the view is the folder's, then let the section show the reason.
      void get().load(chapterId)
      throw e
    }
  },
}))
