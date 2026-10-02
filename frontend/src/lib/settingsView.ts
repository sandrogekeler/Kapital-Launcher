import type { AppSettings, EngineInfo, InstanceReport, Theme } from '../types'

/**
 * What the settings screen says about each field: the value detection
 * resolved when a field is empty, and a line under the field that explains
 * where that value came from. Pure, so the wording is tested once here and
 * the panel only renders it.
 */

/** The three themes in the order the control shows them. */
export const THEME_OPTIONS: readonly { value: Theme; label: string }[] = [
  { value: 'dark', label: 'Dark' },
  { value: 'light', label: 'Light' },
  { value: 'system', label: 'System' },
]

/** `EngineInfo.source` as a phrase: how the Prism in use was found. */
export function sourceLabel(source: string): string {
  switch (source) {
    case 'settings':
      return 'from these settings'
    case 'path':
      return 'on the PATH'
    case 'standard-location':
      return 'at its standard install location'
    case 'flatpak':
      return 'as a Flatpak'
    case 'managed':
      return 'managed by Kapital Launcher'
    default:
      return 'by detection'
  }
}

/** What the executable field shows when it is empty. */
export function executablePlaceholder(engine: EngineInfo | null): string {
  if (engine === null) return 'Checking'
  if (!engine.found) return 'Not found'
  return engine.executable
}

/** The line under the executable field. */
export function executableHint(settings: AppSettings, engine: EngineInfo | null): string {
  const set = settings.prismExecutable.trim() !== ''
  if (engine === null) return 'Looking for Prism Launcher.'
  if (!engine.found) {
    return set
      ? 'Nothing runs at this path. Clear it to detect Prism again.'
      : 'Prism Launcher was not found. Pick its program, or get Prism from the Play bar.'
  }
  if (set && engine.source !== 'settings') {
    return `This path was not found, so Prism ${sourceLabel(engine.source)} is used instead.`
  }
  return set
    ? 'Prism runs from this path.'
    : `Detected ${sourceLabel(engine.source)}. Leave empty to keep detecting.`
}

/** What the data root field shows when it is empty. */
export function rootPlaceholder(instances: InstanceReport | null): string {
  return instances?.root || "Prism's own data folder"
}

/** The line under the data root field. */
export function rootHint(settings: AppSettings, engine: EngineInfo | null): string {
  if (engine?.source === 'managed') {
    return 'The managed Prism keeps its data in its own folder. This setting applies to a Prism you install yourself.'
  }
  return settings.prismRoot.trim() !== ''
    ? 'Prism is started with this folder as its data root.'
    : 'Where Prism keeps its instances. Leave empty for the default.'
}

/** A packOverrides map with one chapter's entry set, or removed when the address is empty. */
export function withPackOverride(
  overrides: Record<string, string> | undefined,
  chapterId: string,
  address: string,
): Record<string, string> | undefined {
  const next = { ...overrides }
  if (address.trim() === '') delete next[chapterId]
  else next[chapterId] = address.trim()
  return Object.keys(next).length === 0 ? undefined : next
}
