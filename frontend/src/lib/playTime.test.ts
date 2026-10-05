import { describe, expect, it } from 'vitest'
import { formatDuration, lastPlayedLabel, playShare, totalSeconds } from './playTime'

describe('formatDuration', () => {
  it('writes hours and minutes, dropping what is zero', () => {
    expect(formatDuration(63 * 3600 + 40 * 60)).toBe('63 h 40 min')
    expect(formatDuration(5 * 3600)).toBe('5 h')
    expect(formatDuration(40 * 60)).toBe('40 min')
    expect(formatDuration(3600 + 59)).toBe('1 h')
  })

  it('rounds down to whole minutes and names the first minute', () => {
    expect(formatDuration(119)).toBe('1 min')
    expect(formatDuration(59)).toBe('< 1 min')
    expect(formatDuration(1)).toBe('< 1 min')
  })

  it('reads nothing, a negative or a bad number as none', () => {
    expect(formatDuration(0)).toBe('0 min')
    expect(formatDuration(-5)).toBe('0 min')
    expect(formatDuration(Number.NaN)).toBe('0 min')
  })
})

describe('lastPlayedLabel', () => {
  // Built from local fields, so the calendar days hold in any time zone.
  const at = (day: number, hour: number, minute = 0) =>
    new Date(2026, 9, day, hour, minute).getTime()
  const now = at(5, 12)

  it('counts local calendar days, not 24 hour spans', () => {
    expect(lastPlayedLabel(at(5, 0, 1), now)).toBe('Last played today')
    expect(lastPlayedLabel(at(4, 23, 50), at(5, 0, 5))).toBe('Last played yesterday')
    expect(lastPlayedLabel(at(4, 8), now)).toBe('Last played yesterday')
    expect(lastPlayedLabel(at(2, 20), now)).toBe('Last played 3 days ago')
    expect(lastPlayedLabel(new Date(2026, 7, 5, 12).getTime(), now)).toBe('Last played 61 days ago')
  })

  it('says so when the game was never started, and treats the future as today', () => {
    expect(lastPlayedLabel(0, now)).toBe('Not played yet')
    expect(lastPlayedLabel(-1, now)).toBe('Not played yet')
    expect(lastPlayedLabel(at(9, 12), now)).toBe('Last played today')
  })
})

describe('totalSeconds and playShare', () => {
  it('sums the chapters, ignoring a negative', () => {
    expect(
      totalSeconds([
        { chapterId: 'a', totalSeconds: 100, lastLaunchMs: 0 },
        { chapterId: 'b', totalSeconds: 50, lastLaunchMs: 0 },
        { chapterId: 'c', totalSeconds: -9, lastLaunchMs: 0 },
      ]),
    ).toBe(150)
    expect(totalSeconds([])).toBe(0)
  })

  it('gives each chapter its whole percentage of the total', () => {
    expect(playShare(50, 200)).toBe(25)
    expect(playShare(200, 200)).toBe(100)
    expect(playShare(0, 200)).toBe(0)
    expect(playShare(5, 0)).toBe(0)
  })

  it('keeps a chapter that played a little visible, and caps at the whole', () => {
    expect(playShare(1, 100_000)).toBe(1)
    expect(playShare(300, 200)).toBe(100)
  })
})
