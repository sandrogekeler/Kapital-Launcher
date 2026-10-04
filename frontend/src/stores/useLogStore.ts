import { create } from 'zustand'
import type { LiveLogEvent, RunLog, RunLogText } from '../types'
import { errMsg, hasWailsBridge } from '../lib/ipc'
import { LIVE_KEY, addLiveLines, defaultLog, joinLines, logKey } from '../lib/runLogs'
import { isActive, useGameStore } from './useGameStore'
import { GetRunLogs, ReadRunLog, StopLiveLog, WatchLiveLog } from '../../wailsjs/go/main/App'
import { ClipboardSetText, EventsOff, EventsOn } from '../../wailsjs/runtime/runtime'

/** The event Go emits for what the live log follower reads. Same string as services.EventLiveLog. */
export const EVENT_LOG_LIVE = 'log:live'

/**
 * The logs page's data (issue 155): the list of a chapter's game logs and crash
 * reports and the one file opened from it. Only the lazy page imports this
 * store, and the page empties it on the way out, because what is held here is
 * a stretch of a game log (already masked by Go) and is meant to live only as
 * long as the page does. Every read is Go's; nothing here polls.
 *
 * The page shows one of two things: a file's chunk, read once, or the live log,
 * which Go follows while it is chosen and sends as `log:live` events. Choosing
 * the live log starts that follower and the listener for it; choosing a file,
 * closing the page and a chapter's change end both.
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
  /** The chapter whose live log is shown, or null when a file is. */
  live: string | null
  /** The live log's lines, masked by Go, the newest LIVE_MAX_LINES of them. */
  liveLines: readonly string[]
  /**
   * Reads the chapter's list, then opens what a player wants first: the live
   * log while the chapter's game is starting or running, else the most recent run.
   */
  load: (chapterId: string) => Promise<void>
  /** Opens the file at its end. A refusal is shown in the viewer, not thrown. */
  open: (chapterId: string, log: RunLog) => Promise<void>
  /**
   * Shows the live log: Go follows latest.log and the lines it sends are appended
   * as they come. A refusal is shown in the viewer, not thrown.
   */
  watch: (chapterId: string) => Promise<void>
  /** Takes a `log:live` event in; one of another chapter is dropped. */
  receiveLive: (event: LiveLogEvent) => void
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
  live: null,
  liveLines: [],
} as const

// An answer that arrives after the page moved on (another file chosen, the page
// closed) is dropped: each start of work takes a number, and only the latest
// number's answer is kept.
let epoch = 0

// Events that arrive while the live log's first text is still on its way. Go
// emits only after it started the follower, so they belong after that text.
let pendingLive: LiveLogEvent[] | null = null

export const useLogStore = create<LogStore>((set, get) => {
  // Ends the live log, if it is the one shown: the listener, then the follower in
  // Go. The Go call is a request to stop; nothing waits on it.
  const leaveLive = () => {
    const chapterId = get().live
    if (chapterId === null) return
    pendingLive = null
    set({ live: null, liveLines: [] })
    if (!hasWailsBridge()) return
    EventsOff(EVENT_LOG_LIVE)
    StopLiveLog(chapterId).catch((e) => console.warn('stop live log', errMsg(e)))
  }

  return {
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
        if (mine !== epoch) return
        set({ logs })
        const first = defaultLog(logs)
        if (isActive(useGameStore.getState().states[chapterId]?.phase)) await get().watch(chapterId)
        else if (first) await get().open(chapterId, first)
      } catch (e) {
        if (mine === epoch) set({ listError: errMsg(e) })
      }
    },

    open: async (chapterId, log) => {
      leaveLive()
      const mine = ++epoch
      set({ selected: log, opened: null, reading: true, readError: null })
      try {
        const text = (await ReadRunLog(chapterId, log.kind, log.name, 0)) as RunLogText
        if (mine === epoch) set({ opened: text, reading: false })
      } catch (e) {
        if (mine === epoch) set({ readError: errMsg(e), reading: false })
      }
    },

    watch: async (chapterId) => {
      leaveLive()
      const mine = ++epoch
      pendingLive = []
      set({
        live: chapterId,
        liveLines: [],
        selected: null,
        opened: null,
        reading: true,
        readError: null,
      })
      // Listening first, so a line Go sends while the first text is on its way is kept.
      if (hasWailsBridge()) EventsOn(EVENT_LOG_LIVE, (e: LiveLogEvent) => get().receiveLive(e))
      try {
        const text = (await WatchLiveLog(chapterId)) as RunLogText
        if (mine !== epoch) return
        let lines = addLiveLines([], text.text, true)
        for (const e of pendingLive ?? []) lines = addLiveLines(lines, e.lines, e.reset)
        pendingLive = null
        set({ liveLines: lines, reading: false })
      } catch (e) {
        if (mine !== epoch) return
        // Nothing is being followed, so there is nothing to listen to.
        pendingLive = null
        if (hasWailsBridge()) EventsOff(EVENT_LOG_LIVE)
        set({ readError: errMsg(e), reading: false })
      }
    },

    // A payload names its chapter and is filed under it: one without a chapter,
    // or for another one, is not this page's.
    receiveLive: (event) => {
      if (!event?.chapterId || event.chapterId !== get().live) return
      if (pendingLive) {
        pendingLive.push(event)
        return
      }
      set((s) => ({ liveLines: addLiveLines(s.liveLines, event.lines, event.reset) }))
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
      const text = get().live !== null ? joinLines(get().liveLines) : get().opened?.text
      if (!text) return
      if (!hasWailsBridge()) throw new Error('Text can only be copied from the app window.')
      if (!(await ClipboardSetText(text))) throw new Error('The text could not be copied.')
    },

    clear: () => {
      epoch++
      leaveLive()
      set({ ...EMPTY })
    },
  }
})

/** The key of what the dropdown shows chosen: the live log, or the file shown or being read. */
export const selectOpenKey = (s: LogStore): string | null =>
  s.live !== null ? LIVE_KEY : s.selected ? logKey(s.selected) : null
