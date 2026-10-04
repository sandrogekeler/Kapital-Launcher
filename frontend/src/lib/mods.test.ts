import { describe, expect, it } from 'vitest'
import type { ChapterMods, ModFile } from '../types'
import { disabledNames, filterMods, modLabel, withJars, withJarsSet } from './mods'

const dh = 'DistantHorizons-3.3.3-1.21.1-fabric-neoforge.jar'
const cw = 'colorwheel-neoforge-1.3.0+mc1.21.1.jar'
const jade = 'Jade-1.21.1-NeoForge-15.1.jar'
const mods: ModFile[] = [
  { name: cw, disabled: false, size: 100 },
  { name: dh, disabled: true, size: 200 },
  { name: jade, disabled: false, size: 300 },
]

describe('mods helpers', () => {
  it('lists the names that are off', () => {
    expect(disabledNames(mods)).toEqual([dh])
    expect(disabledNames([])).toEqual([])
  })

  it('builds the whole list SetModsDisabled takes, adding and removing jars', () => {
    expect(withJars(mods, [jade], true).sort()).toEqual([dh, jade].sort())
    expect(withJars(mods, [dh], false)).toEqual([])
    // A jar already off, or already on, changes nothing.
    expect(withJars(mods, [dh], true)).toEqual([dh])
    expect(withJars(mods, [jade], false)).toEqual([dh])
    expect(withJars(mods, [cw, jade], true).sort()).toEqual([cw, dh, jade].sort())
  })

  it('names a mod without its extension', () => {
    expect(modLabel(dh)).toBe('DistantHorizons-3.3.3-1.21.1-fabric-neoforge')
    expect(modLabel('x.jar.jar')).toBe('x.jar')
    expect(modLabel('readme')).toBe('readme')
  })

  it('filters by every word of the query, in any case, and a blank query keeps all', () => {
    expect(filterMods(mods, '')).toEqual(mods)
    expect(filterMods(mods, '   ')).toEqual(mods)
    expect(filterMods(mods, 'distant').map((m) => m.name)).toEqual([dh])
    expect(filterMods(mods, 'NEOFORGE 1.21').map((m) => m.name)).toEqual(
      [cw, dh, jade].filter((n) => /neoforge/i.test(n)),
    )
    expect(filterMods(mods, 'nothing like it')).toEqual([])
    // A new array: the caller may sort it.
    expect(filterMods(mods, '')).not.toBe(mods)
  })

  it('shows a flip at once and recomputes each toggle from it', () => {
    const view: ChapterMods = {
      chapterId: 'frangfurd',
      mods,
      running: false,
      toggles: [
        { name: 'Distant Horizons', jarPrefix: 'DistantHorizons-', jars: [dh], disabled: true },
        { name: 'Colorwheel', jarPrefix: 'colorwheel-neoforge-', jars: [cw], disabled: false },
        { name: 'Create Better FPS', jarPrefix: 'createbetterfps-', jars: [], disabled: false },
      ],
    }
    const off = withJarsSet(view, [cw], true)
    expect(off.mods.find((m) => m.name === cw)?.disabled).toBe(true)
    expect(off.toggles.map((t) => t.disabled)).toEqual([true, true, false])
    const on = withJarsSet(view, [dh], false)
    expect(on.toggles.map((t) => t.disabled)).toEqual([false, false, false])
    // A toggle that matches no jar is never off, and the original is not changed.
    expect(view.mods[0]?.disabled).toBe(false)
    expect(off.running).toBe(false)
  })
})
