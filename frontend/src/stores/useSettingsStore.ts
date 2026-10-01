import { create } from 'zustand'
import type { AppSettings, Theme } from '../types'
import { errMsg, hasWailsBridge, readOr } from '../lib/ipc'
import { CopyRedactedLog, GetSettings, SaveSettings } from '../../wailsjs/go/main/App'

/**
 * The persisted settings. Defaults mirror models.DefaultSettings in Go, which
 * is what a real install reads on first run.
 */
export const DEFAULT_SETTINGS: AppSettings = {
  theme: 'dark',
  prismExecutable: '',
  prismRoot: '',
  profileName: '',
  lastChapter: '',
}

const THEMES: readonly Theme[] = ['dark', 'light', 'system']

interface SettingsStore {
  settings: AppSettings
  loaded: boolean
  error: string | null
  load: () => Promise<void>
  update: (patch: Partial<AppSettings>) => Promise<void>
  /** Puts the redacted log tail on the clipboard (#84); resolves to the lines copied. */
  copyLog: () => Promise<number>
  clearError: () => void
}

export const useSettingsStore = create<SettingsStore>((set, get) => ({
  settings: DEFAULT_SETTINGS,
  loaded: false,
  error: null,

  load: async () => {
    const raw = await readOr(GetSettings, DEFAULT_SETTINGS)
    set({ settings: sanitize(raw), loaded: true })
  },

  // Optimistic, then persisted. With a bridge present a rejection is a real
  // failed write: revert, record, rethrow so a caller can react. With none,
  // nothing was ever going to persist and the preview stays usable.
  update: async (patch) => {
    const before = get().settings
    const next = sanitize({ ...before, ...patch } as AppSettings)
    set({ settings: next, error: null })
    try {
      await SaveSettings(next)
    } catch (e) {
      if (!hasWailsBridge()) return
      set({ settings: before, error: errMsg(e) })
      throw e
    }
  },

  // The log and the clipboard are Go's. With no bridge there is neither, and
  // saying so beats a TypeError from the binding.
  copyLog: async () => {
    if (!hasWailsBridge()) throw new Error('The log can only be copied from the app window.')
    return CopyRedactedLog()
  },

  clearError: () => set({ error: null }),
}))

/** Coerce whatever came back into values the UI can render. */
export function sanitize(s: AppSettings): AppSettings {
  return {
    ...DEFAULT_SETTINGS,
    ...s,
    theme: (THEMES as readonly string[]).includes(s.theme) ? s.theme : DEFAULT_SETTINGS.theme,
  }
}
