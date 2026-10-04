import type { ChapterMods, ModFile } from '../types'

/** The jar names switched off in a chapter's mods folder, sorted as Go sorts them. */
export const disabledNames = (mods: readonly ModFile[]): string[] =>
  mods.filter((m) => m.disabled).map((m) => m.name)

/**
 * The list SetModsDisabled takes after `jars` are switched off or on: the names already
 * off, with the jars added when they go off and taken away when they come on. Names are
 * the folder's own, so Go finds each one there.
 */
export function withJars(
  mods: readonly ModFile[],
  jars: readonly string[],
  off: boolean,
): string[] {
  const next = new Set(disabledNames(mods))
  for (const jar of jars) {
    if (off) next.add(jar)
    else next.delete(jar)
  }
  return [...next]
}

/** What a jar is called on screen: its file name without `.jar`. */
export const modLabel = (name: string): string => name.replace(/\.jar$/, '')

/** The mods whose name has every word of `query` in it, in any case; all of them for a blank query. */
export function filterMods(mods: readonly ModFile[], query: string): ModFile[] {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean)
  if (words.length === 0) return [...mods]
  return mods.filter((m) => {
    const name = m.name.toLowerCase()
    return words.every((w) => name.includes(w))
  })
}

/**
 * The view as it will be once `off` is set on `jars`, before Go has answered: the mods flipped,
 * and each toggle recomputed from them. Shown at once and replaced by Go's own answer.
 */
export function withJarsSet(view: ChapterMods, jars: readonly string[], off: boolean): ChapterMods {
  const hit = new Set(jars)
  const mods = view.mods.map((m) => (hit.has(m.name) ? { ...m, disabled: off } : m))
  const state = new Map(mods.map((m) => [m.name, m.disabled]))
  return {
    ...view,
    mods,
    toggles: view.toggles.map((t) => ({
      ...t,
      disabled: t.jars.length > 0 && t.jars.every((j) => state.get(j) === true),
    })),
  }
}
