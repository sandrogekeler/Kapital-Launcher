import type { RunLog } from '../types'
import type { SelectOption } from '../components/ui/Select'

/**
 * When a file was written, in the player's locale. A date Go sent that does not
 * parse is shown as it came, so an entry is never blank.
 */
export function formatWhen(iso: string, locale?: string): string {
  const at = new Date(iso)
  if (Number.isNaN(at.getTime())) return iso
  return at.toLocaleString(locale, { dateStyle: 'medium', timeStyle: 'short' })
}

/** The file the game is writing now or wrote last, which is not dated. */
export const isLatest = (log: RunLog): boolean => log.kind === 'log' && log.name === 'latest.log'

/** The key that tells one file from another across both kinds. */
export const logKey = (log: Pick<RunLog, 'kind' | 'name'>): string => `${log.kind}:${log.name}`

/** The value of the entry that follows the game's log as it is written, which no file has. */
export const LIVE_KEY = 'live'

/**
 * The entries of the logs dropdown, in the order a player looks for them: the
 * live log, the most recent run, the dated runs newest first, then the crash
 * reports. Everything is a log, so no entry says so. A run with a crash report
 * carries the Crashed mark. `logs` is Go's list, newest first.
 */
export function logOptions(logs: readonly RunLog[], locale?: string): SelectOption[] {
  const mark = (log: RunLog) => (log.crashed ? 'Crashed' : undefined)
  const when = (log: RunLog) => formatWhen(log.modifiedAt, locale)
  return [
    { value: LIVE_KEY, label: 'Live log' },
    ...logs
      .filter(isLatest)
      .map((l) => ({ value: logKey(l), label: 'Most recent', note: when(l), mark: mark(l) })),
    ...logs
      .filter((l) => l.kind === 'log' && !isLatest(l))
      .map((l) => ({ value: logKey(l), label: when(l), mark: mark(l) })),
    ...logs
      .filter((l) => l.kind === 'crash')
      .map((l) => ({ value: logKey(l), label: 'Crash report', note: when(l) })),
  ]
}

/** The log a page opens on when the game is not running: the most recent run, else the newest entry. */
export function defaultLog(logs: readonly RunLog[]): RunLog | undefined {
  return logs.find(isLatest) ?? logs.find((l) => l.kind === 'log') ?? logs[0]
}

/** The most lines of the live log held at once; the oldest go first. */
export const LIVE_MAX_LINES = 5000

/** The lines of a text, without the empty one after a final newline. */
export function splitLines(text: string): string[] {
  if (text === '') return []
  const lines = text.split('\n')
  if (lines[lines.length - 1] === '') lines.pop()
  return lines
}

/** The live log's lines with more added, or replaced by them, and cut to the newest LIVE_MAX_LINES. */
export function addLiveLines(have: readonly string[], text: string, reset: boolean): string[] {
  const next = reset ? splitLines(text) : have.concat(splitLines(text))
  return next.length > LIVE_MAX_LINES ? next.slice(next.length - LIVE_MAX_LINES) : next
}

/** Lines as the viewer and Copy show them: one per line, a newline after the last. */
export const joinLines = (lines: readonly string[]): string =>
  lines.length === 0 ? '' : `${lines.join('\n')}\n`
