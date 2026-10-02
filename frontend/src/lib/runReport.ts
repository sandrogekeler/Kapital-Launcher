import type { GameState, RunReport } from '../types'

/** Phases a report names the end of a run by: the ones whose time is when it stopped. */
const ENDINGS: readonly string[] = ['crashed', 'failed', 'closed']

/** Seconds as a person reads them: one decimal under ten, whole after. */
export function seconds(ms: number): string {
  const s = Math.max(0, ms) / 1000
  return `${s < 10 ? s.toFixed(1) : Math.round(s)} s`
}

/** Milliseconds from Play to the run's current phase; null when either time is missing or odd. */
function endMs(game: GameState): number | null {
  const from = Date.parse(game.startedAt)
  const to = Date.parse(game.since)
  if (Number.isNaN(from) || Number.isNaN(to) || to < from) return null
  return to - from
}

/**
 * The timeline in one row: when each phase was reached and when the run ended,
 * "mods 4.2 s, window 9.8 s, crashed 31 s". Starting is Play itself and says
 * nothing; a run that failed before any game log is then just "failed 12 s".
 * Null when nothing is known.
 */
export function timeline(report: RunReport): string | null {
  const parts = report.phases
    .filter((p) => p.phase !== 'starting')
    .map((p) => `${p.phase} ${seconds(p.ms)}`)
  const end = endMs(report.game)
  if (ENDINGS.includes(report.game.phase) && end !== null) {
    parts.push(`${report.game.phase} ${seconds(end)}`)
  }
  return parts.length > 0 ? parts.join(', ') : null
}
