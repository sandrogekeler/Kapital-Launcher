import { create } from 'zustand'
import type { RunLog, RunLogText } from '../types'
import { errMsg, hasWailsBridge } from '../lib/ipc'
import { logKey } from '../lib/runLogs'
import { GetRunLogs, ReadRunLog } from '../../wailsjs/go/main/App'
import { ClipboardSetText } from '../../wailsjs/runtime/runtime'

/**
 * The logs page's data (issue 155): the list of a chapter's game logs and crash
 * reports and the one file opened from it. Only the lazy page imports this
 * store, and the page empties it on the way out, because what is held here is
 * a stretch of a game log (already masked by Go) and is meant to live only as
 * long as the page does. Every read is Go's; nothing here polls.
 */
interface LogStore {
  /** The chapter's files, newest first; null before the first read and with no bridge. */
  logs: RunLog[] | null
  /** True when there is no Wails bridge (the browser-only preview), so nothing can be read. */
  unavailable: boolean
  listError: string | null
  /** The file chosen in the list, which is `opened` once Go has read it. */
  selected: RunLog | null
  opened: RunLogText | null
  /** Whether a read of the selected file, or of the chunk before it, is in flight. */
  reading: boolean
  readError: string | null
  load: (chapterId: string) => Promise<void>
  /** Opens the file at its end. A refusal is shown in the viewer, not thrown. */
  open: (chapterId: string, log: RunLog) => Promise<void>
  /** Reads the stretch before what is shown and puts it in front. */
  loadEarlier: (chapterId: string) => Promise<void>
  /** Puts the shown text on the clipboard; rejects with the reason when it could not. */
  copy: () => Promise<void>
  clear: () => void
}

const EMPTY = {
  logs: null,
  unavailable: false,
  listError: null,
  selected: null,
  opened: null,
  reading: false,
  readError: null,
} as const

// An answer that arrives after the page moved on (another file chosen, the page
// closed) is dropped: each start of work takes a number, and only the latest
// number's answer is kept.
let epoch = 0

export const useLogStore = create<LogStore>((set, get) => ({
  ...EMPTY,

  load: async (chapterId) => {
    const mine = ++epoch
    if (!hasWailsBridge()) {
      set({ ...EMPTY, unavailable: true })
      return
    }
    set({ ...EMPTY })
    try {
      const logs = (await GetRunLogs(chapterId)) as RunLog[]
      if (mine === epoch) set({ logs })
    } catch (e) {
      if (mine === epoch) set({ listError: errMsg(e) })
    }
  },

  open: async (chapterId, log) => {
    const mine = ++epoch
    set({ selected: log, opened: null, reading: true, readError: null })
    try {
      const text = (await ReadRunLog(chapterId, log.kind, log.name, 0)) as RunLogText
      if (mine === epoch) set({ opened: text, reading: false })
    } catch (e) {
      if (mine === epoch) set({ readError: errMsg(e), reading: false })
    }
  },

  loadEarlier: async (chapterId) => {
    const { opened, reading } = get()
    if (!opened || reading || !opened.truncated) return
    const mine = ++epoch
    set({ reading: true, readError: null })
    try {
      const chunk = (await ReadRunLog(
        chapterId,
        opened.kind,
        opened.name,
        opened.offset,
      )) as RunLogText
      if (mine !== epoch) return
      set({
        opened: {
          ...chunk,
          text: chunk.text + opened.text,
          lines: chunk.lines + opened.lines,
        },
        reading: false,
      })
    } catch (e) {
      if (mine === epoch) set({ readError: errMsg(e), reading: false })
    }
  },

  // The clipboard is Go's, reached through the runtime binding. With no bridge
  // there is none, and saying so beats a TypeError from it.
  copy: async () => {
    const text = get().opened?.text
    if (!text) return
    if (!hasWailsBridge()) throw new Error('Text can only be copied from the app window.')
    if (!(await ClipboardSetText(text))) throw new Error('The text could not be copied.')
  },

  clear: () => {
    epoch++
    set({ ...EMPTY })
  },
}))

/** The key of the file shown, or of the one being read. */
export const selectOpenKey = (s: LogStore): string | null =>
  s.selected ? logKey(s.selected) : null
