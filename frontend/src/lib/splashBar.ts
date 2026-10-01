import type { GameState } from '../types'

/** The phases the card's bar walks through, in order; `resources` is the handover. */
const STEPS = ['starting', 'mods', 'window', 'resources'] as const

/** Where the bar starts and heads at a phase change, and how long the ease takes. */
export interface BarPlan {
  /** Fill, 0 to 1, the bar jumps to when the phase begins. */
  from: number
  /** Fill it eases towards, 0 to 1, the next phase's place on the way. */
  to: number
  /** Milliseconds the ease takes: the estimated gap to the next phase. */
  ms: number
}

/**
 * The determinate bar's plan for a phase, or null when the bar is
 * indeterminate: no estimate (a first start has no history), none for the
 * handover, or a phase the bar does not cover.
 *
 * The estimate is milliseconds from Play to each phase, and the card goes away
 * at `resources`, so that one is 100%. A phase's own fraction is where the bar
 * jumps to; the next phase's is where it eases to, over the estimated gap. A
 * phase the history lacks borrows the nearest earlier one (or Play) as its own
 * start and the nearest later one as its target, so the bar still moves.
 */
export function barPlan(
  phase: GameState['phase'] | undefined,
  estimate: GameState['estimate'],
): BarPlan | null {
  const total = estimate?.resources
  if (!total || total <= 0) return null
  const at = STEPS.indexOf(phase as (typeof STEPS)[number])
  if (at < 0) return null
  if (STEPS[at] === 'resources') return { from: 1, to: 1, ms: 0 }

  const time = (step: (typeof STEPS)[number]) => (step === 'starting' ? 0 : estimate?.[step])
  const earlier = STEPS.slice(0, at + 1)
    .reverse()
    .map(time)
  const later = STEPS.slice(at + 1).map(time)
  const start = earlier.find((t) => t !== undefined) ?? 0
  const end = later.find((t) => t !== undefined) ?? total
  // A run that was slower than its mean must not run the bar backwards.
  const reach = Math.max(end, start)
  const frac = (ms: number) => Math.min(1, Math.max(0, ms / total))
  return { from: frac(start), to: frac(reach), ms: reach - start }
}
