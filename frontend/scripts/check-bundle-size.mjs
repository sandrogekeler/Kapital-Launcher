#!/usr/bin/env node
//
// The entry chunk's gzip budget. A launcher opens, shows three chapters and a
// Play button; nothing about that needs a large bundle, and the budget is what
// keeps a future dependency from quietly changing that. A ratchet: lower it as
// the chunk shrinks, raise it only with a reason written beside the number.
//
// Run with: pnpm check-bundle
import { readdir, readFile } from 'node:fs/promises'
import { gzipSync } from 'node:zlib'
import path from 'node:path'
import { ensureFreshDist } from './lib/dist-freshness.mjs'

const BUDGET_KB = 90

const DIST_ASSETS = await ensureFreshDist()
const files = (await readdir(DIST_ASSETS)).filter((f) => f.endsWith('.js'))
const entry = files.find((f) => /^index-/.test(f))
if (!entry) {
  console.error(`check-bundle: no entry chunk (index-*.js) in dist/assets: ${files.join(', ')}`)
  process.exit(1)
}
const bytes = gzipSync(await readFile(path.join(DIST_ASSETS, entry))).length
const kb = bytes / 1024
const verdict = kb <= BUDGET_KB ? 'ok' : 'OVER BUDGET'
console.log(`check-bundle: ${entry} ${kb.toFixed(1)} KB gzip (budget ${BUDGET_KB} KB) ${verdict}`)
if (kb > BUDGET_KB) process.exit(1)
