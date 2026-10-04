import { describe, expect, it } from 'vitest'
import { formatBytes } from './bytes'

describe('formatBytes', () => {
  it('reads as B, KB, MB and GB with a decimal under ten', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(1023)).toBe('1023 B')
    expect(formatBytes(1024)).toBe('1.0 KB')
    expect(formatBytes(150 * 1024)).toBe('150 KB')
    expect(formatBytes(2_500_000)).toBe('2.4 MB')
    expect(formatBytes(12 * 1024 ** 3)).toBe('12 GB')
  })
})
