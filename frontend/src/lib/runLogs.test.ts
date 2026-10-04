import { describe, expect, it } from 'vitest'
import {
  LIVE_KEY,
  LIVE_MAX_LINES,
  addLiveLines,
  defaultLog,
  formatWhen,
  isLatest,
  joinLines,
  logKey,
  logOptions,
  splitLines,
} from './runLogs'
import type { RunLog } from '../types'

const log = (over: Partial<RunLog>): RunLog => ({
  kind: 'log',
  name: '2026-10-03-1.log.gz',
  modifiedAt: '2026-10-03T12:00:00Z',
  size: 1,
  crashed: false,
  ...over,
})

describe('formatWhen', () => {
  it("writes the date and time in the player's locale", () => {
    const iso = '2026-10-03T12:34:00Z'
    expect(formatWhen(iso, 'en-GB')).toBe(
      new Date(iso).toLocaleString('en-GB', { dateStyle: 'medium', timeStyle: 'short' }),
    )
    expect(formatWhen(iso, 'de-DE')).not.toBe(formatWhen(iso, 'en-US'))
  })

  it('shows a date that does not parse as it came', () => {
    expect(formatWhen('yesterday')).toBe('yesterday')
  })
})

describe('keys', () => {
  it('tells latest.log from a crash report of the same name', () => {
    expect(isLatest(log({ name: 'latest.log' }))).toBe(true)
    expect(isLatest(log({ kind: 'crash', name: 'latest.log' }))).toBe(false)
  })

  it('keys a file by kind and name', () => {
    expect(logKey(log({ name: 'latest.log' }))).toBe('log:latest.log')
    expect(logKey(log({ kind: 'crash', name: 'latest.log' }))).toBe('crash:latest.log')
  })
})

describe('logOptions', () => {
  const latest = log({ name: 'latest.log', modifiedAt: '2026-10-04T10:00:00Z' })
  const crash = log({ kind: 'crash', name: 'crash-1.txt', modifiedAt: '2026-10-03T21:59:00Z' })
  const newer = log({ name: '2026-10-03-2.log.gz', modifiedAt: '2026-10-03T22:00:00Z' })
  const older = log({
    name: '2026-10-02-1.log.gz',
    modifiedAt: '2026-10-02T20:00:00Z',
    crashed: true,
  })
  // Go's order: newest first, the two kinds mixed by time.
  const listed = [latest, newer, crash, older]

  it('puts the live log first, then the most recent run, the dated runs and the crash reports', () => {
    const options = logOptions(listed, 'en-GB')
    expect(options.map((o) => o.value)).toEqual([
      LIVE_KEY,
      'log:latest.log',
      'log:2026-10-03-2.log.gz',
      'log:2026-10-02-1.log.gz',
      'crash:crash-1.txt',
    ])
    expect(options[0]).toEqual({ value: LIVE_KEY, label: 'Live log' })
    expect(options[1]).toMatchObject({
      label: 'Most recent',
      note: formatWhen(latest.modifiedAt, 'en-GB'),
    })
    // A dated run is its date and time and nothing more.
    expect(options[2]).toMatchObject({ label: formatWhen(newer.modifiedAt, 'en-GB') })
    expect(options[2]?.note).toBeUndefined()
    expect(options[4]).toMatchObject({
      label: 'Crash report',
      note: formatWhen(crash.modifiedAt, 'en-GB'),
    })
  })

  it('marks the run a crash belongs to and nothing else, and never says Log of a file', () => {
    const options = logOptions(listed, 'en-GB')
    expect(options.filter((o) => o.mark).map((o) => o.value)).toEqual(['log:2026-10-02-1.log.gz'])
    expect(options[3]?.mark).toBe('Crashed')
    for (const o of options.slice(1)) expect(`${o.label} ${o.note ?? ''}`).not.toMatch(/\blog\b/i)
  })

  it('offers the live log alone when nothing is written yet', () => {
    expect(logOptions([]).map((o) => o.value)).toEqual([LIVE_KEY])
  })
})

describe('defaultLog', () => {
  it('is the most recent run, else the newest log, else the newest entry', () => {
    const latest = log({ name: 'latest.log' })
    const dated = log({})
    const crash = log({ kind: 'crash', name: 'c.txt' })
    expect(defaultLog([dated, latest])).toBe(latest)
    expect(defaultLog([crash, dated])).toBe(dated)
    expect(defaultLog([crash])).toBe(crash)
    expect(defaultLog([])).toBeUndefined()
  })
})

describe('the live log lines', () => {
  it('splits a text into its lines without the empty one after the last newline', () => {
    expect(splitLines('')).toEqual([])
    expect(splitLines('a\nb\n')).toEqual(['a', 'b'])
    expect(splitLines('a\n\nb\n')).toEqual(['a', '', 'b'])
    expect(splitLines('a')).toEqual(['a'])
  })

  it('joins lines back with a newline after the last', () => {
    expect(joinLines([])).toBe('')
    expect(joinLines(['a', 'b'])).toBe('a\nb\n')
  })

  it('appends, replaces on a reset, and keeps only the newest lines', () => {
    expect(addLiveLines(['a'], 'b\nc\n', false)).toEqual(['a', 'b', 'c'])
    expect(addLiveLines(['a', 'b'], 'x\n', true)).toEqual(['x'])
    expect(addLiveLines(['a', 'b'], '', true)).toEqual([])
    const have = Array.from({ length: LIVE_MAX_LINES }, (_, i) => `l${i}`)
    const next = addLiveLines(have, 'new1\nnew2\n', false)
    expect(next).toHaveLength(LIVE_MAX_LINES)
    expect(next.slice(-2)).toEqual(['new1', 'new2'])
    expect(next[0]).toBe('l2')
  })
})
