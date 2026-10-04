import { useCallback, useState } from 'react'

/**
 * One page laid over the chapter: what is open, and whether it is meant to be
 * showing. Hiding does not remove it: the page stays until its exit animation
 * ends (`done`, from `PageLayer`), so it can slide out. Showing again, even
 * mid-exit, brings it back.
 */
export function usePageSlot<T>() {
  const [slot, setSlot] = useState<{ page: T; open: boolean } | null>(null)
  const show = useCallback((page: T) => setSlot({ page, open: true }), [])
  const hide = useCallback(() => setSlot((s) => (s?.open ? { ...s, open: false } : s)), [])
  const done = useCallback(() => setSlot((s) => (s && !s.open ? null : s)), [])
  return { slot, show, hide, done }
}
