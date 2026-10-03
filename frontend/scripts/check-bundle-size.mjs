#!/usr/bin/env node
//
// The launcher's JavaScript gzip budget. A launcher opens, shows three chapters
// and a Play button; nothing about that needs a large bundle, and the budget is
// what keeps a future dependency from quietly changing that. A ratchet: lower
// it as the bundle shrinks, raise it only with a reason written beside the
// number.
//
// The loading card's page (#97) is a second entry that shares React with the
// launcher, so the build splits the shared code into a chunk of its own and
// the launcher's entry chunk alone is small. What the launcher loads is its
// entry, index.html's script, and the chunks that page preloads: that stream,
// gzipped as one the way the single entry chunk was, is what is measured. The
// card's own entry (splash-*.js) is not part of it.
//
// Run with: pnpm check-bundle
import { readFile } from 'node:fs/promises'
import { gzipSync } from 'node:zlib'
import path from 'node:path'
import { ensureFreshDist } from './lib/dist-freshness.mjs'

// 91 since the corner notices (2026-10-03): the launcher's errors left the
// action bar for a component of their own, always loaded, which is 0.4 KB
// gzipped. Measured 90.3 KB that day, from 89.9 KB before it.
// 92 since the slideshow (2026-10-03, #142): the chapter's picture and wiki post
// cycle together, which every chapter view shows from the first paint (Drift,
// the slide pairing, the rotation). Measured 91.2 KB.
const BUDGET_KB = 92

const DIST_ASSETS = await ensureFreshDist()
const DIST = path.dirname(DIST_ASSETS)
const html = await readFile(path.join(DIST, 'index.html'), 'utf8')

// The entry script first, then each chunk the page preloads, as Vite writes them.
const entry = /<script[^>]*type="module"[^>]*src="\/assets\/(index-[^"]+\.js)"/.exec(html)?.[1]
if (!entry) {
  console.error('check-bundle: index.html has no entry script (assets/index-*.js)')
  process.exit(1)
}
const shared = [
  ...html.matchAll(/<link[^>]*rel="modulepreload"[^>]*href="\/assets\/([^"]+\.js)"/g),
].map((m) => m[1])
const files = [entry, ...shared]
const stream = Buffer.concat(
  await Promise.all(files.map((f) => readFile(path.join(DIST_ASSETS, f)))),
)
const kb = gzipSync(stream).length / 1024
const verdict = kb <= BUDGET_KB ? 'ok' : 'OVER BUDGET'
console.log(
  `check-bundle: ${files.join(' + ')} ${kb.toFixed(1)} KB gzip (budget ${BUDGET_KB} KB) ${verdict}`,
)
if (kb > BUDGET_KB) process.exit(1)
