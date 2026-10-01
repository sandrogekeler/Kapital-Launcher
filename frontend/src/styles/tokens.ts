/* GENERATED FILE. DO NOT EDIT.
 *
 * Produced by frontend/scripts/gen-tokens.mjs from design/tokens.json. A hand
 * edit here is reverted by the next `pnpm gen:tokens` and never reaches the
 * TypeScript or Go copies of the same values. Change design/tokens.json, then
 * regenerate.
 */

export type ThemeMode = 'dark' | 'light'

/** Every chapter the token set names an accent for. The manifest's chapter ids must be among these. */
export const CHAPTER_IDS = ['luxemburg', 'lichdenstein', 'frangfurd'] as const
export type ChapterId = (typeof CHAPTER_IDS)[number]

/** The chapter accents as hex, per theme, for the rare place a colour is needed as a value. */
export const CHAPTER_ACCENTS: Record<ThemeMode, Record<ChapterId, string>> = {
  dark: {
  luxemburg: '#e0aa4e',
  lichdenstein: '#5cbfa2',
  frangfurd: '#7cbfe0',
  },
  light: {
  luxemburg: '#8a6512',
  lichdenstein: '#1a6b55',
  frangfurd: '#1c6a8e',
  },
}

/** Window geometry, in px. main.go reads the same numbers from backend/design/design_gen.go. */
export const WINDOW = { width: 1280, height: 800, minWidth: 1024, minHeight: 640 } as const

export const LAYOUT = {
  sidebar: 264,
  hero: 440,
  titlebar: 40,
  icon: { sm: 16, md: 18, stroke: 1.2 },
  splash: { width: 720, height: 405 },
} as const
