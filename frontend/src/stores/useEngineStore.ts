import { create } from 'zustand'
import type {
  ChapterSettings,
  ChapterSettingsInfo,
  EngineInfo,
  InstanceReport,
  PrismInstallProgress,
  PrismRelease,
} from '../types'
import { errMsg, hasWailsBridge, readOr } from '../lib/ipc'
import {
  GetChapterSettings,
  GetEngine,
  GetInstances,
  GetPrismRelease,
  InstallChapter,
  InstallPrism,
  LaunchChapter,
  RefreshEngine,
  SaveChapterSettings,
} from '../../wailsjs/go/main/App'
import { EventsOff, EventsOn } from '../../wailsjs/runtime/runtime'

/** Go emits one per install step. Same string as services.EventPrismInstall. */
export const EVENT_PRISM_INSTALL = 'prism:install'

/**
 * What is known about Prism, which chapters' instances it has, and the launch
 * action. The engine is the one thing the app cannot work without, so its
 * absence is a state the UI shows rather than an error it swallows.
 */
interface EngineStore {
  engine: EngineInfo | null
  instances: InstanceReport | null
  launching: string | null
  /** The chapter whose instance is being written, while InstallChapter runs. */
  installing: string | null
  /** The chapter installed last, until it is played: its first Play downloads the pack. */
  installedNow: string | null
  error: string | null
  /** Prism's latest release, when the launcher could install or update it; null when unknown. */
  release: PrismRelease | null
  /** The latest prism:install step while an install runs, and its outcome after. */
  install: PrismInstallProgress | null
  /** Each chapter's instance settings, by chapter id, once read (#36). */
  chapterSettings: Record<string, ChapterSettingsInfo | undefined>
  load: () => Promise<void>
  loadChapterSettings: (chapterId: string) => Promise<void>
  /** Writes the settings into the instance; rejects with Go's reason, which the panel shows. */
  saveChapterSettings: (chapterId: string, settings: ChapterSettings) => Promise<void>
  loadRelease: () => Promise<void>
  installPrism: () => Promise<void>
  listenInstall: () => () => void
  receiveInstall: (p: PrismInstallProgress) => void
  loadInstances: () => Promise<void>
  refresh: () => Promise<void>
  launch: (chapterId: string) => Promise<void>
  installChapter: (chapterId: string) => Promise<void>
  clearError: () => void
}

export const useEngineStore = create<EngineStore>((set, get) => ({
  engine: null,
  instances: null,
  launching: null,
  installing: null,
  installedNow: null,
  error: null,
  release: null,
  install: null,
  chapterSettings: {},

  // A read of the instance's own file: nothing to show without a bridge or
  // an instance, and the panel says so.
  loadChapterSettings: async (chapterId) => {
    const info = await readOr(() => GetChapterSettings(chapterId), undefined)
    set((s) => ({ chapterSettings: { ...s.chapterSettings, [chapterId]: info } }))
  },

  // A write into the instance: Go validates, refuses a running game, and
  // answers with what it wrote, so the panel shows the file's truth.
  saveChapterSettings: async (chapterId, settings) => {
    const info = await SaveChapterSettings(chapterId, settings)
    set((s) => ({ chapterSettings: { ...s.chapterSettings, [chapterId]: info } }))
  },

  // What installing or updating Prism would fetch. A read: without a bridge,
  // or when GitHub cannot be reached, there is simply nothing to offer.
  loadRelease: async () => {
    set({ release: await readOr(GetPrismRelease, null) })
  },

  // A write the player approved. Progress arrives as events; the promise
  // settles the outcome, so a failure is recorded even if no event came.
  installPrism: async () => {
    set({
      install: { phase: 'downloading', received: 0, total: get().release?.size ?? 0, error: '' },
    })
    try {
      await InstallPrism()
      set({ install: { phase: 'done', received: 0, total: 0, error: '' } })
      await get().load()
      await get().loadRelease()
    } catch (e) {
      set({ install: { phase: 'failed', received: 0, total: 0, error: errMsg(e) } })
    }
  },

  // One listener for the app's lifetime, like server:status (.claude/rules/ipc.md).
  listenInstall: () => {
    if (!hasWailsBridge()) return () => undefined
    EventsOn(EVENT_PRISM_INSTALL, (p: PrismInstallProgress) => get().receiveInstall(p))
    return () => EventsOff(EVENT_PRISM_INSTALL)
  },

  // A late event must not reopen a finished install: once the promise has
  // settled it as done or failed, only a new install moves it again.
  receiveInstall: (p) => {
    const current = get().install
    if (!p?.phase || current === null || current.phase === 'done' || current.phase === 'failed')
      return
    set({ install: p })
  },

  load: async () => {
    set({ engine: await readOr(GetEngine, null) })
    await get().loadInstances()
  },

  // A read of the disk as it is now: instances appear when the user imports
  // one in Prism, so App calls this again when the window regains focus.
  loadInstances: async () => {
    set({ instances: await readOr(GetInstances, null) })
  },

  refresh: async () => {
    try {
      set({ engine: await RefreshEngine(), error: null })
    } catch (e) {
      set({ error: errMsg(e) })
      return
    }
    await get().loadInstances()
  },

  // A launch is a write: with no bridge nothing was going to start, so the
  // rejection is shown as what it is rather than kept optimistic.
  launch: async (chapterId) => {
    // "Prism is ready, sign in on first play" has said its piece once Play is pressed.
    if (get().install?.phase === 'done') set({ install: null })
    set({ launching: chapterId, installedNow: null, error: null })
    try {
      await LaunchChapter(chapterId)
    } catch (e) {
      set({ error: errMsg(e) })
    } finally {
      set({ launching: null })
    }
  },

  // A write like launch: Go answers with the instances read again, so the
  // button turns into Play from what is on disk, not from an assumption.
  installChapter: async (chapterId) => {
    set({ installing: chapterId, installedNow: null, error: null })
    try {
      const instances = await InstallChapter(chapterId)
      set({ instances, installedNow: instances.present[chapterId] ? chapterId : null })
    } catch (e) {
      set({ error: errMsg(e) })
    } finally {
      set({ installing: null })
    }
  },

  clearError: () => set({ error: null }),
}))

/** The pack URL the chapter's instance syncs from, when the launcher made it. */
export const selectInstancePack = (chapterId: string) => (s: EngineStore) =>
  s.instances?.packUrl?.[chapterId]

/** What the chapter's instance takes on disk, or undefined when it is missing or unmeasured. */
export const selectInstanceSize = (chapterId: string) => (s: EngineStore) =>
  s.instances?.sizeBytes?.[chapterId]

/** Whether the chapter's instance exists: true, false, or undefined for unknown. */
export const selectInstalled = (chapterId: string) => (s: EngineStore) =>
  s.instances?.present[chapterId]
