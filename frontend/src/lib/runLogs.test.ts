import { describe, expect, it } from 'vitest'
import { formatBytes, formatWhen, isLatest, kindLabel, logKey } from './runLogs'
import type { RunLog } from '../types'

const log = (over: Partial<RunLog>): RunLog => ({
  kind: 'log',
  name: '2026-10-03-1.log.gz',
  modifiedAt: '2026-10-03T12:00:00Z',
  size: 1,
  crashed: false,
  ...over,
})

describe('formatBytes', () => {
  it('reads as B, KB and MB with a decimal under ten', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(1023)).toBe('1023 B')
    expect(formatBytes(1024)).toBe('1.0 KB')
    expect(formatBytes(4096)).toBe('4.0 KB')
    expect(formatBytes(150 * 1024)).toBe('150 KB')
    expect(formatBytes(2_500_000)).toBe('2.4 MB')
    expect(formatBytes(3 * 1024 ** 3)).toBe('3.0 GB')
    expect(formatBytes(5000 * 1024 ** 3)).toBe('5000 GB')
  })
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

describe('what a row is called', () => {
  it('names latest.log as the current or most recent run, and the rest by kind', () => {
    expect(isLatest(log({ name: 'latest.log' }))).toBe(true)
    expect(isLatest(log({ kind: 'crash', name: 'latest.log' }))).toBe(false)
    expect(kindLabel(log({ name: 'latest.log' }))).toBe('Log, current or most recent run')
    expect(kindLabel(log({}))).toBe('Log')
    expect(kindLabel(log({ kind: 'crash', name: 'crash-x.txt' }))).toBe('Crash report')
  })

  it('keys a file by kind and name', () => {
    expect(logKey(log({ name: 'latest.log' }))).toBe('log:latest.log')
    expect(logKey(log({ kind: 'crash', name: 'latest.log' }))).toBe('crash:latest.log')
  })
})
