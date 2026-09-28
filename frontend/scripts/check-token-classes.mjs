#!/usr/bin/env node
//
// Guards against a token-named Tailwind class that compiles to nothing.
//
// Tailwind v4 resolves each utility against a specific theme namespace, and a
// name that lands in the wrong one produces no rule at all: the element
// silently keeps a default, nothing looks wrong in review, and the token is
// dead. Konnekt shipped `duration-fast` that way for months. This builds every
// class name the token source could produce, keeps the ones src/ uses, and
// asserts each has a rule in the built CSS.
//
// Run with: pnpm check-tokens
import { readdir, readFile } from 'node:fs/promises'
import path from 'node:path'
import { ensureFreshDist } from './lib/dist-freshness.mjs'

const here = import.meta.dirname
const SOURCE = path.join(here, '..', '..', 'design', 'tokens.json')
const SRC_DIR = path.join(here, '..', 'src')
const DIST_ASSETS = await ensureFreshDist()

const COLOR_PREFIXES = [
  'bg',
  'text',
  'border',
  'border-t',
  'border-r',
  'border-b',
  'border-l',
  'outline',
  'ring',
  'fill',
  'stroke',
  'decoration',
  'placeholder',
]
const RADIUS_PREFIXES = ['rounded', 'rounded-t', 'rounded-b', 'rounded-l', 'rounded-r']

// Mirrors gen-tokens.mjs's alias table. Duplicated on purpose: importing the
// generator would run it, and a check that shares its subject's code cannot
// disagree with it.
const UTILITY_ALIAS = {
  'bg-sunken': 'sunken',
  bg: 'canvas',
  'bg-raised': 'raised',
  'bg-raised-2': 'raised-2',
  'bg-hover': 'hover',
}

const src = JSON.parse(await readFile(SOURCE, 'utf8'))

/** @type {{ cls: string, group: string }[]} */
const candidates = []
const add = (cls, group) => candidates.push({ cls, group })

for (const [group, tokens] of Object.entries(src.color)) {
  for (const name of Object.keys(tokens)) {
    const utility = group === 'chapter' ? `chapter-${name}` : (UTILITY_ALIAS[name] ?? name)
    for (const prefix of COLOR_PREFIXES) add(`${prefix}-${utility}`, 'colour')
  }
}
for (const utility of ['accent', 'accent-wash', 'accent-edge']) {
  for (const prefix of COLOR_PREFIXES) add(`${prefix}-${utility}`, 'colour')
}
for (const name of Object.keys(src.type.size.scale)) add(`text-${name}`, 'type size')
for (const name of Object.keys(src.type.family)) add(`font-${name}`, 'font family')
for (const name of Object.keys(src.radius.scale)) {
  for (const prefix of RADIUS_PREFIXES) add(`${prefix}-${name}`, 'radius')
}
for (const name of Object.keys(src.motion.duration.scale)) {
  add(`duration-${name}`, 'motion')
  add(`delay-${name}`, 'motion')
}
for (const name of Object.keys(src.motion.easing)) add(`ease-${name}`, 'motion')

// A class name's boundary must not treat `-` as a separator, or `text-fg`
// would match inside `text-fg-muted`.
const escape = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
const usedIn = (text, cls) => new RegExp(`(?<![\\w-])${escape(cls)}(?![\\w-])`).test(text)

async function walk(dir) {
  const out = []
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) out.push(...(await walk(full)))
    else if (/\.(tsx?|css)$/.test(entry.name)) out.push(full)
  }
  return out
}

const sources = await Promise.all((await walk(SRC_DIR)).map((f) => readFile(f, 'utf8')))
const allSource = sources.join('\n')
const used = candidates.filter(({ cls }) => usedIn(allSource, cls))

const cssFiles = (await readdir(DIST_ASSETS)).filter((f) => f.endsWith('.css'))
if (cssFiles.length === 0) {
  console.error('check-tokens: no CSS in dist/assets')
  process.exit(1)
}
const builtCss = (
  await Promise.all(cssFiles.map((f) => readFile(path.join(DIST_ASSETS, f), 'utf8')))
).join('\n')

// Tailwind escapes `.` and `/` in emitted selectors; the names here carry neither,
// so a plain `.cls{` or `.cls:` or `.cls,` search is enough. A class used only
// under a variant compiles only in that form, `.hover\:cls:hover{`, so any
// escaped `variant\:` prefixes are allowed in front of the name.
const compiled = (cls) =>
  new RegExp(`\\.(?:[\\w-]+\\\\:)*${escape(cls)}(?=[{:,\\s>])`).test(builtCss)

const dead = used.filter(({ cls }) => !compiled(cls))
if (dead.length > 0) {
  console.error('check-tokens: token-named classes used in src/ but absent from the built CSS:\n')
  for (const { cls, group } of dead) console.error(`  ${cls}  (${group})`)
  console.error(
    '\nEach of these is a utility Tailwind did not generate. Either the token is emitted\n' +
      'into the wrong @theme namespace (fix gen-tokens.mjs) or the class is misspelled.',
  )
  process.exit(1)
}

console.log(`check-tokens: ${used.length} token-named classes in use, all compile.`)
