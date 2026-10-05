import { useSyncExternalStore } from 'react'

const QUERY = '(prefers-reduced-motion: reduce)'

/** The media query, or null where there is none to ask (a test's jsdom). */
const list = (): MediaQueryList | null =>
  typeof window !== 'undefined' && typeof window.matchMedia === 'function'
    ? window.matchMedia(QUERY)
    : null

function subscribe(onChange: () => void): () => void {
  const mql = list()
  mql?.addEventListener('change', onChange)
  return () => mql?.removeEventListener('change', onChange)
}

/** Whether the player asked their system for less motion; follows a change while the app is open. */
export function useReducedMotion(): boolean {
  return useSyncExternalStore(
    subscribe,
    () => list()?.matches ?? false,
    () => false,
  )
}
