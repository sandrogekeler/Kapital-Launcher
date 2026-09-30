import { describe, expect, it } from 'vitest'
import { isLocalPack, packHost, packSourceLine } from './packSource'

const hosted = 'https://kapitel-kapital.pages.dev/frangfurd/pack.toml'

describe('packSource', () => {
  it('tells a local packwiz serve from a hosted pack', () => {
    expect(isLocalPack('http://localhost:8080/pack.toml')).toBe(true)
    expect(isLocalPack('http://127.0.0.1:8080/pack.toml')).toBe(true)
    expect(isLocalPack('http://[::1]:8080/pack.toml')).toBe(true)
    expect(isLocalPack(hosted)).toBe(false)
    expect(isLocalPack('not a url')).toBe(false)
  })

  it('names the host and port', () => {
    expect(packHost('http://localhost:8080/pack.toml')).toBe('localhost:8080')
    expect(packHost('not a url')).toBe('not a url')
  })

  it("marks an instance that does not sync from the manifest's pack", () => {
    expect(packSourceLine('http://localhost:8080/pack.toml', null)).toEqual([
      '● Dev pack',
      'Syncs from localhost:8080',
    ])
    expect(packSourceLine('https://example.pages.dev/pack.toml', hosted)).toEqual([
      '● Other pack',
      'Syncs from example.pages.dev',
    ])
    expect(packSourceLine(hosted, hosted)).toBeNull()
    expect(packSourceLine(undefined, hosted)).toBeNull()
  })
})
