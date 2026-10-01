import { describe, expect, it } from 'vitest'
import { barPlan } from './splashBar'

const est = { mods: 20_000, window: 30_000, resources: 40_000, running: 50_000 }

describe('barPlan', () => {
  it('heads for the next phase from the start of this one, resources being 100%', () => {
    expect(barPlan('starting', est)).toEqual({ from: 0, to: 0.5, ms: 20_000 })
    expect(barPlan('mods', est)).toEqual({ from: 0.5, to: 0.75, ms: 10_000 })
    expect(barPlan('window', est)).toEqual({ from: 0.75, to: 1, ms: 10_000 })
  })

  it('holds full at the handover', () => {
    expect(barPlan('resources', est)).toEqual({ from: 1, to: 1, ms: 0 })
  })

  it('is indeterminate without an estimate or without the handover in it', () => {
    expect(barPlan('starting', undefined)).toBeNull()
    expect(barPlan('mods', {})).toBeNull()
    expect(barPlan('mods', { mods: 5_000 })).toBeNull()
  })

  it('is indeterminate for a phase it does not cover', () => {
    expect(barPlan('idle', est)).toBeNull()
    expect(barPlan('crashed', est)).toBeNull()
    expect(barPlan(undefined, est)).toBeNull()
  })

  it('borrows its neighbours when a phase has no history', () => {
    // No window time: mods heads straight for resources, window starts where mods did.
    const gap = { mods: 10_000, resources: 40_000 }
    expect(barPlan('mods', gap)).toEqual({ from: 0.25, to: 1, ms: 30_000 })
    expect(barPlan('window', gap)).toEqual({ from: 0.25, to: 1, ms: 30_000 })
    expect(barPlan('starting', { resources: 40_000 })).toEqual({ from: 0, to: 1, ms: 40_000 })
  })

  it('never runs backwards when the means disagree', () => {
    expect(barPlan('window', { mods: 20_000, window: 30_000, resources: 25_000 })).toEqual({
      from: 1,
      to: 1,
      ms: 0,
    })
  })
})
