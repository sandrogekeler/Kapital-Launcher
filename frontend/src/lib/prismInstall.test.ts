import { describe, expect, it } from 'vitest'
import { installLine, megabytes } from './prismInstall'

const at = (phase: string, received = 0, total = 0, error = '') => ({
  phase,
  received,
  total,
  error,
})

describe('megabytes', () => {
  it('reads as the sizes GitHub shows', () => {
    expect(megabytes(20396629)).toBe('19.5 MB')
    expect(megabytes(0)).toBe('0.0 MB')
  })
})

describe('installLine', () => {
  it('counts the download up in whole percent, never past 100', () => {
    expect(installLine(at('downloading', 10198315, 20396629), 'Getting')).toEqual([
      '◐ Getting Prism · 50%',
      'Downloading 9.7 of 19.5 MB',
    ])
    expect(installLine(at('downloading', 30, 20), 'Updating')[0]).toBe('◐ Updating Prism · 100%')
    expect(installLine(at('downloading'), 'Getting')[0]).toBe('◐ Getting Prism · 0%')
  })

  it('names each later step, the sign-in when done, and the reason when it fails', () => {
    expect(installLine(at('unpacking'), 'Getting')).toEqual(['◐ Getting Prism', 'Unpacking'])
    expect(installLine(at('verifying'), 'Getting')[1]).toMatch(/signature/)
    expect(installLine(at('done'), 'Getting')[1]).toMatch(/sign in with Microsoft/)
    expect(installLine(at('failed', 0, 0, 'digest mismatch'), 'Getting')).toEqual([
      '○ Could not get Prism',
      'digest mismatch',
    ])
  })
})
