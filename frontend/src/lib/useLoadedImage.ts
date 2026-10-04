import { useEffect, useReducer, useState } from 'react'

/**
 * Pictures decoded in this window, by src: one that is here is drawn at once,
 * with nothing to wait for. Bundled art is added at startup (preloadImages);
 * a wiki picture when it is first shown.
 */
const decoded = new Set<string>()

/** Pictures that failed to load, so a broken one is not tried on every render. */
const failed = new Set<string>()

/**
 * The decoded images themselves, held so the browser keeps them: the bundled
 * art only (titles, icons, the chapters' own pictures), a few hundred KB.
 */
const kept: HTMLImageElement[] = []

/** Loads and decodes `src`, resolving when it can be drawn without a gap. */
function load(src: string, keep = false): Promise<void> {
  if (decoded.has(src)) return Promise.resolve()
  const img = new Image()
  img.src = src
  if (keep) kept.push(img)
  // jsdom has no decode, and loads no images: there it counts as drawn.
  const ready = typeof img.decode === 'function' ? img.decode() : Promise.resolve()
  return ready.then(
    () => {
      decoded.add(src)
    },
    () => {
      failed.add(src)
    },
  )
}

/**
 * Decodes the bundled art once, at startup (the author, issue 172 follow-up),
 * so a title or a chapter's own picture never pops in when a chapter is
 * switched to: by then it is in memory.
 */
export function preloadImages(srcs: readonly (string | undefined)[]): void {
  for (const src of srcs) {
    if (src) void load(src, true)
  }
}

/**
 * The picture to draw for `src`: `src` itself once it is decoded, until then
 * the last picture this component drew, else `fallback`. A caller that
 * animates on a change of what this returns (the hero's Drift) therefore
 * starts its entrance only when the new picture is ready, and never shows a
 * half-loaded one. A picture that fails to load keeps the last one.
 */
export function useLoadedImage(
  src: string | undefined,
  fallback: string | undefined,
): string | undefined {
  const [, loaded] = useReducer((n: number) => n + 1, 0)
  const [last, setLast] = useState<string | undefined>(() =>
    src && decoded.has(src) ? src : fallback,
  )
  useEffect(() => {
    if (!src || decoded.has(src) || failed.has(src)) return
    let live = true
    void load(src).then(() => {
      if (live) loaded()
    })
    return () => {
      live = false
    }
  }, [src])
  const ready = src && decoded.has(src) ? src : undefined
  // The last drawn picture follows what is ready, in render, as Drift's does.
  if (ready && ready !== last) setLast(ready)
  return ready ?? last ?? fallback
}
