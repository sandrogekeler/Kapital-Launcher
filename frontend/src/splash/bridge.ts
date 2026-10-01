import { create } from 'zustand'
import type { GameState } from '../types'

/**
 * The loading card's side of its two-way bridge (#97). The card is a window of
 * its own, with this page in it and no Wails runtime: Go pushes the state with
 * `window.kapitalSplash.update(state)`, and the page asks Go for one of three
 * actions by posting a string through whichever bridge the host gave it. Go
 * refuses anything else (backend/splashhost/protocol.go has the whole
 * protocol), so a new action is added on both sides or not at all.
 */

/** What Go pushes: backend/splashhost.State. */
export interface CardState {
  chapter: { id: string; name: string; packVersion?: string }
  game: GameState
  /** The last copyLog outcome: how many lines went, or why none did. */
  copyLog?: { lines?: number; error?: string }
  /** What a failed action says. */
  error?: string
  /** The player's theme when it is not the system's. */
  theme?: 'dark' | 'light'
}

/** The three things the page may ask Go to do. */
export type CardAction = 'leave' | 'openFolder' | 'copyLog'

interface CardStore {
  state: CardState | null
}

/** The newest state Go pushed; null until the first one, and the card draws nothing till then. */
export const useCardStore = create<CardStore>(() => ({ state: null }))

declare global {
  interface Window {
    kapitalSplash?: { update: (state: unknown) => void }
    chrome?: { webview?: { postMessage: (message: string) => void } }
    webkit?: {
      messageHandlers?: { splash?: { postMessage: (message: string) => void } }
    }
  }
}

/** A push that is not a card's state is dropped: nothing here may throw into the host. */
function isCardState(value: unknown): value is CardState {
  if (typeof value !== 'object' || value === null) return false
  const { chapter, game } = value as Partial<CardState>
  return (
    typeof chapter?.id === 'string' &&
    typeof chapter.name === 'string' &&
    typeof game?.phase === 'string'
  )
}

/** Defines `window.kapitalSplash`, which Go's first push calls. Returns what removes it. */
export function installBridge(): () => void {
  window.kapitalSplash = {
    update: (state) => {
      if (isCardState(state)) useCardStore.setState({ state })
    },
  }
  return () => {
    delete window.kapitalSplash
  }
}

/**
 * Asks Go for an action: the JSON {"action": ...} as a string, through
 * WebView2's bridge on Windows or WebKit's on macOS. With neither (a test, the
 * browser preview) it does nothing.
 */
export function send(action: CardAction): void {
  const message = JSON.stringify({ action })
  if (window.chrome?.webview) window.chrome.webview.postMessage(message)
  else window.webkit?.messageHandlers?.splash?.postMessage(message)
}
