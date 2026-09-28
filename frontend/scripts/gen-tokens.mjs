#!/usr/bin/env node
//
// Generate every derived form of the design tokens from the one source.
//
//   design/tokens.json  ->  frontend/src/styles/tokens.css   (Tailwind theme + themed values)
//                       ->  frontend/src/styles/tokens.ts    (the same values, for code that needs them)
//                       ->  backend/design/design_gen.go     (window geometry, for main.go)
//
// The source is deliberately tech-neutral: colours are { hex, alpha }, easings
// are four numbers, sizes are bare numbers with a unit on the group. Turning
// that into CSS, TypeScript and Go is this file's whole job, so a value is
// written once and every consumer reads the same one.
//
// Run with: pnpm gen:tokens

import { readFileSync, writeFileSync, mkdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname } from 'node:path'

const SOURCE = new URL('../../design/tokens.json', import.meta.url)
const CSS_OUT = new URL('../src/styles/tokens.css', import.meta.url)
const TS_OUT = new URL('../src/styles/tokens.ts', import.meta.url)
const GO_OUT = new URL('../../backend/design/design_gen.go', import.meta.url)

const SUPPORTED_VERSION = 1

// Semantic token name -> the utility suffix Tailwind exposes it under.
// --color-canvas becomes bg-canvas/text-canvas/border-canvas, so this is where
// "bg" (the page ground) becomes "canvas" rather than the unreadable `bg-bg`.
// It lives here rather than in the source because it is a Tailwind concern.
const UTILITY_ALIAS = {
  'bg-sunken': 'sunken',
  bg: 'canvas',
  'bg-raised': 'raised',
  'bg-raised-2': 'raised-2',
  'bg-hover': 'hover',
}

// Tailwind's font-size scale keys. A colour exposed under one of these names
// produces --color-<key>, which generates a text-<key> *colour* utility that
// silently shadows the font-size utility of the same name.
const RESERVED_UTILITY_NAMES = new Set([
  'xs',
  'sm',
  'base',
  'lg',
  'xl',
  '2xl',
  '3xl',
  '4xl',
  '5xl',
  '6xl',
  '7xl',
  '8xl',
  '9xl',
])

// Font family names that must stay unquoted: generic families and system
// keywords are CSS-wide identifiers, not font names. Everything else is quoted.
const BARE_FAMILIES = new Set([
  'serif',
  'sans-serif',
  'monospace',
  'system-ui',
  'ui-serif',
  'ui-sans-serif',
  'ui-monospace',
  '-apple-system',
])

function fail(message) {
  console.error(`gen-tokens: ${message}`)
  process.exit(1)
}

// ── Validation ──────────────────────────────────────────────────────────────
// design/tokens.schema.json is the contract. These are the checks worth
// repeating here: the ones whose failure would otherwise produce plausible CSS
// with the wrong values in it.

function validate(src) {
  if (src.version !== SUPPORTED_VERSION) {
    fail(
      `token source is version ${src.version}, this generator understands ${SUPPORTED_VERSION}. ` +
        `Update the generator rather than the source.`,
    )
  }
  for (const group of ['surface', 'text', 'line', 'chapter', 'status']) {
    if (!src.color?.[group]) fail(`missing color.${group}`)
  }
  for (const group of ['type', 'radius', 'motion', 'layout']) {
    if (!src[group]) fail(`missing ${group}`)
  }
  for (const [group, tokens] of Object.entries(src.color)) {
    for (const [name, token] of Object.entries(tokens)) {
      if (!token.dark?.hex) fail(`color.${group}.${name} has no dark value`)
      for (const mode of ['dark', 'light']) {
        const value = token[mode]
        if (value == null) continue
        if (!/^#[0-9a-f]{6}$/.test(value.hex)) {
          fail(`color.${group}.${name}.${mode}: "${value.hex}" is not a six-digit lowercase hex`)
        }
        if (value.alpha != null && !(value.alpha > 0 && value.alpha < 1)) {
          fail(`color.${group}.${name}.${mode}: alpha ${value.alpha} must be between 0 and 1`)
        }
      }
      if (RESERVED_UTILITY_NAMES.has(utilityName(group, name))) {
        fail(
          `color.${group}.${name} maps to the utility name "${utilityName(group, name)}", one of ` +
            `Tailwind's font-size keys; text-<name> would become a colour rule.`,
        )
      }
    }
  }
  for (const [name, points] of Object.entries(src.motion.easing)) {
    if (points.length !== 4) fail(`motion.easing.${name} must be four numbers`)
  }
  const w = src.layout.window
  if (w.minWidth > w.width || w.minHeight > w.height) {
    fail('layout.window minimum exceeds its default size')
  }
}

// ── Value formatting ────────────────────────────────────────────────────────

function channels(hex) {
  const n = parseInt(hex.slice(1), 16)
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255]
}

function css(color) {
  if (color.alpha == null) return color.hex
  return `rgb(${channels(color.hex).join(' ')} / ${color.alpha})`
}

function utilityName(group, name) {
  if (group === 'chapter') return `chapter-${name}`
  return UTILITY_ALIAS[name] ?? name
}

function fontStack(families) {
  return families.map((f) => (BARE_FAMILIES.has(f) ? f : `'${f}'`)).join(', ')
}

function variation(axes) {
  return Object.entries(axes)
    .map(([axis, value]) => `'${axis}' ${value}`)
    .join(', ')
}

const BANNER = `/* GENERATED FILE. DO NOT EDIT.
 *
 * Produced by frontend/scripts/gen-tokens.mjs from design/tokens.json. A hand
 * edit here is reverted by the next \`pnpm gen:tokens\` and never reaches the
 * TypeScript or Go copies of the same values. Change design/tokens.json, then
 * regenerate.
 */`

// ── CSS emission ────────────────────────────────────────────────────────────

function emitCss(src) {
  const lines = []
  const push = (line = '') => lines.push(line)
  push(BANNER)
  push()
  emitThemeMapping(src, push)
  push()
  emitThemeValues(src, push)
  push()
  emitLayout(src, push)
  emitPalettes(src, lines, push)
  push()
  emitAccents(src, push)
  push()
  return lines.join('\n')
}

function emitThemeMapping(src, push) {
  push(`/* Colours are \`inline\` so a utility resolves straight to the themed custom`)
  push(`   property below: bg-canvas becomes background-color: var(--bg), which is`)
  push(`   what lets a theme or a chapter switch retheme by changing the property. */`)
  push(`@theme inline {`)
  for (const [group, tokens] of Object.entries(src.color)) {
    for (const name of Object.keys(tokens)) {
      const prop = group === 'chapter' ? `chapter-${name}` : name
      push(`  --color-${utilityName(group, name)}: var(--${prop});`)
    }
  }
  push(`  /* The active chapter's accent. Set by [data-chapter] below. */`)
  push(`  --color-accent: var(--accent);`)
  push(`  --color-accent-wash: var(--accent-wash);`)
  push(`  --color-accent-edge: var(--accent-edge);`)
  push(`}`)
}

function emitThemeValues(src, push) {
  push(`/* Everything else is a plain @theme block, deliberately not \`inline\`, so the`)
  push(`   custom property exists for hand-written CSS as well as for the utility. */`)
  push(`@theme {`)
  push(`  /* Type. The scale overrides Tailwind's defaults of the same name, so text-sm`)
  push(`     is this UI's 13px rather than Tailwind's 14px. */`)
  for (const [name, value] of Object.entries(src.type.size.scale)) {
    const size = typeof value === 'number' ? value : value.size
    push(`  --text-${name}: ${size}${src.type.size.unit};`)
    // Tailwind v4 reads these companions when the text-* utility is applied.
    if (typeof value === 'object' && value.lineHeight != null) {
      push(`  --text-${name}--line-height: ${value.lineHeight};`)
    }
    if (typeof value === 'object' && value.tracking != null) {
      push(`  --text-${name}--letter-spacing: ${value.tracking}em;`)
    }
  }
  push()
  for (const [name, families] of Object.entries(src.type.family)) {
    push(`  --font-${name}: ${fontStack(families)};`)
    // Same companion mechanism: font-display applies the variation axes too.
    const axes = src.type.variation?.[name]
    if (axes) push(`  --font-${name}--font-variation-settings: ${variation(axes)};`)
  }
  push()
  push(`  /* Radius. */`)
  for (const [name, value] of Object.entries(src.radius.scale)) {
    push(`  --radius-${name}: ${value}${src.radius.unit};`)
  }
  push()
  push(`  /* Motion. Each duration is emitted twice from one value: --duration-* is what`)
  push(`     hand-written CSS reads, and Tailwind resolves its duration-* utility against`)
  push(`     --transition-duration-*, never --duration-*. */`)
  for (const [name, value] of Object.entries(src.motion.duration.scale)) {
    const ms = `${value}${src.motion.duration.unit}`
    push(`  --duration-${name}: ${ms};`)
    push(`  --transition-duration-${name}: ${ms};`)
  }
  for (const [name, points] of Object.entries(src.motion.easing)) {
    push(`  --ease-${name}: cubic-bezier(${points.join(', ')});`)
  }
  push(`}`)
}

function emitLayout(src, push) {
  push(`/* Layout geometry is not a Tailwind namespace. Read it with the var()`)
  push(`   shorthand, w-(--layout-sidebar), or from style.css. */`)
  push(`:root {`)
  push(`  --layout-sidebar: ${src.layout.sidebar}${src.layout.unit};`)
  push(`  --layout-hero: ${src.layout.hero}${src.layout.unit};`)
  push(`  --layout-titlebar: ${src.layout.titlebar}${src.layout.unit};`)
  push(`}`)
}

// One palette block: every colour token's value for one mode. A null light
// value inherits the dark one; emitting it again would be a second copy to
// keep in step for no behavioural gain.
function emitPalette(src, push, selector, mode) {
  push()
  push(`${selector} {`)
  if (mode === 'light') push(`  color-scheme: light;`)
  for (const [group, tokens] of Object.entries(src.color)) {
    for (const [name, token] of Object.entries(tokens)) {
      const value = token[mode]
      if (value == null) continue
      const prop = group === 'chapter' ? `chapter-${name}` : name
      push(`  --${prop}: ${css(value)};`)
    }
  }
  push(`}`)
}

function emitPalettes(src, lines, push) {
  push()
  push(`/* Dark is the default. Light is opt-in through data-theme; the system`)
  push(`   preference is honoured only when nothing was chosen, and the light block`)
  push(`   is repeated under the media query rather than reordered so an explicit`)
  push(`   choice always wins. */`)
  emitPalette(src, push, `:root,\n[data-theme='dark']`, 'dark')
  emitPalette(src, push, `[data-theme='light']`, 'light')
  push()
  push(`@media (prefers-color-scheme: light) {`)
  const save = lines.length
  emitPalette(src, push, `  :root:not([data-theme='dark'])`, 'light')
  const inner = lines.splice(save).map((l) => (l ? `  ${l}` : l))
  lines.push(...inner.slice(1))
  push(`}`)
}

function emitAccents(src, push) {
  push(`/* The chapter accent. Set once by data-chapter on the root; read by anything`)
  push(`   inside through --accent, or the accent, accent-wash and accent-edge`)
  push(`   utilities. The first chapter is the fallback so nothing renders unaccented. */`)
  const chapterIds = Object.keys(src.color.chapter)
  push(`:root {`)
  push(`  --accent: var(--chapter-${chapterIds[0]});`)
  push(`  --accent-wash: color-mix(in srgb, var(--accent) 12%, transparent);`)
  push(`  --accent-edge: color-mix(in srgb, var(--accent) 42%, transparent);`)
  push(`}`)
  for (const id of chapterIds) {
    push(`[data-chapter='${id}'] {`)
    push(`  --accent: var(--chapter-${id});`)
    push(`}`)
  }
}

// ── TS emission ─────────────────────────────────────────────────────────────

function emitTs(src) {
  const chapterIds = Object.keys(src.color.chapter)
  const accents = (mode) =>
    chapterIds
      .map((id) => `  ${id}: '${(src.color.chapter[id][mode] ?? src.color.chapter[id].dark).hex}',`)
      .join('\n')
  const w = src.layout.window
  return `${BANNER}

export type ThemeMode = 'dark' | 'light'

/** Every chapter the token set names an accent for. The manifest's chapter ids must be among these. */
export const CHAPTER_IDS = [${chapterIds.map((id) => `'${id}'`).join(', ')}] as const
export type ChapterId = (typeof CHAPTER_IDS)[number]

/** The chapter accents as hex, per theme, for the rare place a colour is needed as a value. */
export const CHAPTER_ACCENTS: Record<ThemeMode, Record<ChapterId, string>> = {
  dark: {
${accents('dark')}
  },
  light: {
${accents('light')}
  },
}

/** Window geometry, in px. main.go reads the same numbers from backend/design/design_gen.go. */
export const WINDOW = { width: ${w.width}, height: ${w.height}, minWidth: ${w.minWidth}, minHeight: ${w.minHeight} } as const

export const LAYOUT = { sidebar: ${src.layout.sidebar}, hero: ${src.layout.hero}, titlebar: ${src.layout.titlebar} } as const
`
}

// ── Go emission ─────────────────────────────────────────────────────────────

function emitGo(src) {
  const w = src.layout.window
  const bg = channels(src.color.surface.bg.dark.hex)
  return `// Code generated by frontend/scripts/gen-tokens.mjs from design/tokens.json. DO NOT EDIT.

// Package design carries the design values the Go side reads. The frontend
// reads the same values from src/styles/tokens.ts, both from one source, so
// the window main.go opens is the window the layout was drawn for.
package design

// Window geometry, in logical pixels.
const (
	WindowWidth     = ${w.width}
	WindowHeight    = ${w.height}
	WindowMinWidth  = ${w.minWidth}
	WindowMinHeight = ${w.minHeight}
)

// WindowBackground is the page ground (color.surface.bg, dark), painted by the
// shell before the first frame so a slow WebView never flashes white.
var WindowBackground = [3]uint8{${bg.join(', ')}}

// ChapterIDs is every chapter the token set names an accent for, in order.
// The launcher manifest may not name a chapter outside this list.
var ChapterIDs = []string{${Object.keys(src.color.chapter)
    .map((id) => `"${id}"`)
    .join(', ')}}
`
}

// ── Main ────────────────────────────────────────────────────────────────────

const src = JSON.parse(readFileSync(SOURCE, 'utf8'))
validate(src)

for (const [out, content] of [
  [CSS_OUT, emitCss(src)],
  [TS_OUT, emitTs(src)],
  [GO_OUT, emitGo(src)],
]) {
  mkdirSync(dirname(fileURLToPath(out)), { recursive: true })
  writeFileSync(out, content)
  console.log(`gen-tokens: wrote ${fileURLToPath(out)}`)
}
