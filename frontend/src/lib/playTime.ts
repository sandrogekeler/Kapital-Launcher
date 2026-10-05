/**
 * Prism's play time as the account page words it (issue 192). Pure helpers: Go reads the two
 * numbers (`GetPlayTime`), and these turn them into the strings and shares the page shows.
 */
import type { PlayTime } from '../types'

const MINUTE = 60
const HOUR = 60 * MINUTE

/**
 * A length of play in the page's words: "63 h 40 min", "5 h", "40 min". Whole minutes, rounded
 * down as a clock reads; some seconds but less than a minute is "< 1 min", and none at all is
 * "0 min". A negative or non-finite value is none.
 */
export function formatDuration(seconds: number): string {
  const s = Number.isFinite(seconds) && seconds > 0 ? Math.floor(seconds) : 0
  if (s === 0) return '0 min'
  if (s < MINUTE) return '< 1 min'
  const hours = Math.floor(s / HOUR)
  const minutes = Math.floor((s % HOUR) / MINUTE)
  if (hours === 0) return `${minutes} min`
  return minutes === 0 ? `${hours} h` : `${hours} h ${minutes} min`
}

/** Midnight of the local calendar day `ms` falls in, as a UTC-based day number. */
function dayNumber(ms: number): number {
  const d = new Date(ms)
  return Math.round(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()) / 86_400_000)
}

/**
 * "Last played today", "yesterday" or "N days ago", by local calendar days, so a game closed at
 * 23:50 reads "yesterday" a quarter of an hour later and not "today" for a day. No launch time
 * (0, which Prism leaves a never-started instance with) is "Not played yet"; a time in the
 * future, from a clock that moved, reads as today.
 */
export function lastPlayedLabel(lastLaunchMs: number, nowMs: number): string {
  if (!Number.isFinite(lastLaunchMs) || lastLaunchMs <= 0) return 'Not played yet'
  const days = dayNumber(nowMs) - dayNumber(lastLaunchMs)
  if (days <= 0) return 'Last played today'
  if (days === 1) return 'Last played yesterday'
  return `Last played ${days} days ago`
}

/** The seconds across a list of chapters. */
export function totalSeconds(times: readonly PlayTime[]): number {
  return times.reduce((sum, t) => sum + Math.max(0, t.totalSeconds), 0)
}

/**
 * One chapter's share of the total as a whole percentage for its bar, 0 when nothing has been
 * played. A chapter that has played at all keeps at least 1, so its bar is not invisible next to
 * a long one.
 */
export function playShare(seconds: number, total: number): number {
  if (!(total > 0) || !(seconds > 0)) return 0
  return Math.min(100, Math.max(1, Math.round((seconds / total) * 100)))
}
