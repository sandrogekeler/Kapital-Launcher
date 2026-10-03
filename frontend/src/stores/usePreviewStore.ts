import { create } from 'zustand'
import type { PreviewSituation } from '../types'
import { hasWailsBridge, readOr } from '../lib/ipc'
import { ClearPreviews, GetPreviewSituations, StartPreview } from '../../wailsjs/go/main/App'
import { useEngineStore } from './useEngineStore'
import { useGameStore } from './useGameStore'

/**
 * The developer previews (#124): screens a run only shows when something goes
 * wrong, faked from the Developer section of Settings. Go pushes synthetic
 * state through the real paths (a `game:state` event, the loading card, the
 * pack, instance, engine and release reads, `prism:install`), so the real
 * components render their real copy. This store asks for one and reads what the
 * screens show again; the state itself lives in the stores that own it.
 *
 * Only the Developer section uses it, which is loaded on demand, so the store
 * is loaded with it and stays out of the launcher's first paint.
 */
interface PreviewStore {
  /** The fixed list of situations, from Go; empty without a bridge. */
  situations: PreviewSituation[]
  load: () => Promise<void>
  /** Shows a situation for a chapter. A write: a refusal is rethrown for the section to show. */
  start: (chapterId: string, situation: PreviewSituation) => Promise<void>
  /** Ends every preview and reads the real state again. */
  clear: () => Promise<void>
}

/** The install outcomes a preview leaves behind, which are not news after it. */
const FINISHED = ['done', 'failed']

/**
 * Reads again what a preview changes that no event carries: the engine, the
 * release and the instances with the pack state. The release goes first, so a
 * made-up one never outlives its preview; it is read again where the launcher
 * would read it (App does the same on the engine).
 */
async function readViews() {
  useEngineStore.setState((s) => ({
    release: null,
    install: s.install && FINISHED.includes(s.install.phase) ? null : s.install,
  }))
  await useEngineStore.getState().load()
  const { engine, loadRelease } = useEngineStore.getState()
  if (engine && (!engine.found || engine.source === 'managed')) await loadRelease()
}

export const usePreviewStore = create<PreviewStore>((set) => ({
  situations: [],

  load: async () => {
    const raw: unknown = await readOr(GetPreviewSituations, [])
    set({ situations: Array.isArray(raw) ? (raw as PreviewSituation[]) : [] })
  },

  start: async (chapterId, situation) => {
    if (!hasWailsBridge()) throw new Error('Previews need the desktop app.')
    await StartPreview(chapterId, situation.id)
    await readViews()
    // The made-up install is the situation: it plays on its own, over
    // prism:install, and fails.
    if (situation.playsInstall) void useEngineStore.getState().installPrism()
  },

  clear: async () => {
    if (!hasWailsBridge()) throw new Error('Previews need the desktop app.')
    await ClearPreviews()
    await readViews()
    await useGameStore.getState().load()
  },
}))
