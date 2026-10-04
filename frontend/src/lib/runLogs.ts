import type { RunLog } from '../types'

/** Bytes as a person reads them: B under a kilobyte, then KB, MB, one decimal under ten. */
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${Math.max(0, bytes)} B`
  const units = ['KB', 'MB', 'GB']
  let value = bytes / 1024
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit++
  }
  return `${value < 10 ? value.toFixed(1) : Math.round(value)} ${units[unit]}`
}

/**
 * When a file was written, in the player's locale. A date Go sent that does not
 * parse is shown as it came, so a row is never blank.
 */
export function formatWhen(iso: string, locale?: string): string {
  const at = new Date(iso)
  if (Number.isNaN(at.getTime())) return iso
  return at.toLocaleString(locale, { dateStyle: 'medium', timeStyle: 'short' })
}

/** The file the game is writing now or wrote last, which is not dated. */
export const isLatest = (log: RunLog): boolean => log.kind === 'log' && log.name === 'latest.log'

/** What a row is called: the kind, and for latest.log which run it is. */
export function kindLabel(log: RunLog): string {
  if (log.kind === 'crash') return 'Crash report'
  return isLatest(log) ? 'Log, current or most recent run' : 'Log'
}

/** The key that tells one file from another across both kinds. */
export const logKey = (log: Pick<RunLog, 'kind' | 'name'>): string => `${log.kind}:${log.name}`
