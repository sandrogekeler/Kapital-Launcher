#!/usr/bin/env node
//
// The built pages' Content-Security-Policy, held to the manifest (issue 161).
//
// index.html's `frame-src` is written at build time from the chapters' `map`
// addresses in data/launcher.json (vite.config.ts, mapFrameSrc). This reads the
// output back and fails unless that directive is exactly the set of those
// addresses' origins, so a map origin the manifest does not name, a wildcard, a
// scheme alone or a stale build can never ship. It computes the expectation
// with `new URL` and not with the plugin's own pattern, so the two do not share
// a mistake. splash.html frames nothing and must still say `frame-src 'none'`,
// and neither page may carry a looser source anywhere else in the policy.
//
// Run with: pnpm check-csp
import { readFile } from 'node:fs/promises'
import path from 'node:path'
import { ensureFreshDist } from './lib/dist-freshness.mjs'

const DIST = path.dirname(await ensureFreshDist())
const manifest = JSON.parse(
  await readFile(path.resolve(import.meta.dirname, '../../data/launcher.json'), 'utf8'),
)

const policyOf = (html, page) => {
  const found = [
    ...html.matchAll(/<meta[^>]*http-equiv="Content-Security-Policy"[^>]*content="([^"]*)"/gi),
  ]
  if (found.length !== 1)
    fail(`${page}: expected one Content-Security-Policy tag, found ${found.length}`)
  return new Map(
    found[0][1]
      .split(';')
      .map((d) => d.trim().split(/\s+/))
      .filter((d) => d[0])
      .map(([name, ...sources]) => [name, sources]),
  )
}

function fail(message) {
  console.error(`check-csp: ${message}`)
  process.exit(1)
}

const expected = [
  ...new Set(manifest.chapters.filter((c) => c.map != null).map((c) => new URL(c.map).origin)),
].sort()

const index = policyOf(await readFile(path.join(DIST, 'index.html'), 'utf8'), 'index.html')
const frames = index.get('frame-src') ?? []
const want = expected.length ? expected : ["'none'"]
if (JSON.stringify([...frames].sort()) !== JSON.stringify([...want].sort())) {
  fail(
    `index.html frame-src is [${frames.join(' ')}], the manifest's map origins are [${want.join(' ')}]`,
  )
}
for (const source of frames) {
  if (source !== "'none'" && !/^https?:\/\/[a-z0-9.-]+\.tun\.ply\.gg:[0-9]{1,5}$/.test(source)) {
    fail(`index.html frame-src carries ${source}, which is not a tunnel origin with a port`)
  }
}
// Nothing else in the policy may have been loosened to make the frame work.
for (const [name, sources] of index) {
  if (name !== 'frame-src' && sources.some((s) => /^https?:/.test(s) || s === '*')) {
    fail(`index.html ${name} carries ${sources.join(' ')}`)
  }
}
if (index.get('script-src')?.join(' ') !== "'self'") fail('index.html script-src is not just self')

const splash = policyOf(await readFile(path.join(DIST, 'splash.html'), 'utf8'), 'splash.html')
if ((splash.get('frame-src') ?? []).join(' ') !== "'none'")
  fail("splash.html frame-src is not 'none'")

console.log(`check-csp: index.html frame-src ${frames.join(' ')}; splash.html 'none' ok`)
