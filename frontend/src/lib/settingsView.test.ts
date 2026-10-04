import { describe, expect, it } from 'vitest'
import type { AppSettings, EngineInfo } from '../types'
import { DEFAULT_SETTINGS } from '../stores/useSettingsStore'
import {
  executableHint,
  executablePlaceholder,
  rootHint,
  rootPlaceholder,
  sourceLabel,
  withPackOverride,
} from './settingsView'

const found = (source: string, executable = 'C:\\Prism\\prismlauncher.exe'): EngineInfo => ({
  found: true,
  executable,
  version: '11.1.1',
  root: '',
  source,
})
const missing: EngineInfo = { found: false, executable: '', version: '', root: '', source: '' }
const withExe: AppSettings = { ...DEFAULT_SETTINGS, prismExecutable: 'D:\\old\\prismlauncher.exe' }

describe('settingsView', () => {
  it('names every detection source', () => {
    for (const s of ['settings', 'path', 'standard-location', 'flatpak', 'managed', 'other']) {
      expect(sourceLabel(s)).not.toBe('')
    }
    expect(sourceLabel('managed')).toContain('Kapital Launcher')
  })

  it('shows the detected program in the empty executable field', () => {
    expect(executablePlaceholder(null)).toBe('Checking')
    expect(executablePlaceholder(missing)).toBe('Not found')
    expect(executablePlaceholder(found('path'))).toBe('C:\\Prism\\prismlauncher.exe')
  })

  it('speaks under the executable field only when something is off', () => {
    expect(executableHint(DEFAULT_SETTINGS, null)).toContain('Looking')
    expect(executableHint(DEFAULT_SETTINGS, missing)).toContain('not found')
    expect(executableHint(withExe, missing)).toContain('Nothing runs')
    // A stale path on file: detection fell past it.
    expect(executableHint(withExe, found('managed'))).toContain('used instead')
    // Found where the field or its placeholder says: nothing to add.
    expect(executableHint(DEFAULT_SETTINGS, found('standard-location'))).toBeNull()
    expect(executableHint(withExe, found('settings'))).toBeNull()
  })

  it('shows the resolved root and says when the managed Prism ignores it', () => {
    expect(rootPlaceholder(null)).toBe("Prism's own data folder")
    expect(
      rootPlaceholder({ root: 'C:\\data', dir: '', present: {}, packUrl: {}, sizeBytes: {} }),
    ).toBe('C:\\data')
    expect(rootHint(found('managed'))).toContain('managed Prism')
    expect(rootHint(found('path'))).toBeNull()
    expect(rootHint(null)).toBeNull()
  })

  it('sets and clears one chapter in packOverrides', () => {
    const one = withPackOverride(undefined, 'frangfurd', ' http://localhost:8080/pack.toml ')
    expect(one).toEqual({ frangfurd: 'http://localhost:8080/pack.toml' })
    expect(withPackOverride(one, 'luxemburg', 'http://127.0.0.1:9000/pack.toml')).toEqual({
      frangfurd: 'http://localhost:8080/pack.toml',
      luxemburg: 'http://127.0.0.1:9000/pack.toml',
    })
    expect(withPackOverride(one, 'frangfurd', '')).toBeUndefined()
    expect(withPackOverride(undefined, 'frangfurd', '  ')).toBeUndefined()
  })
})
