// Shared precondition for the checks that read `frontend/dist` rather than
// `frontend/src`: check-token-classes.mjs and check-bundle-size.mjs.
//
// A check that reads a stale build gives a confident wrong answer in both
// directions: a dist older than src reports live classes as dead, and a dist
// that still carries a rule the sources no longer produce reports a regression
// as green. So the precondition is enforced rather than assumed: compare
// mtimes, and build when the build is missing or older than anything that
// feeds it. A fresh dist means no work, which is why this is free in CI.
import { readdir, stat } from 'node:fs/promises'
import { spawnSync } from 'node:child_process'
import path from 'node:path'

const FRONTEND = path.resolve(import.meta.dirname, '..', '..')
const DIST_ASSETS = path.join(FRONTEND, 'dist', 'assets')

// Everything whose change can change the built output. design/tokens.json is
// the root of the generated token layer and lives at the repo root.
const INPUTS = [
  path.join(FRONTEND, 'src'),
  path.join(FRONTEND, 'index.html'),
  path.join(FRONTEND, 'splash.html'),
  path.join(FRONTEND, 'vite.config.ts'),
  path.join(FRONTEND, 'package.json'),
  path.join(FRONTEND, '..', 'design', 'tokens.json'),
  path.join(FRONTEND, '..', 'data', 'launcher.json'),
]

async function newestMtime(target) {
  let info
  try {
    info = await stat(target)
  } catch {
    return 0
  }
  if (!info.isDirectory()) return info.mtimeMs
  const entries = await readdir(target, { withFileTypes: true })
  const times = await Promise.all(entries.map((e) => newestMtime(path.join(target, e.name))))
  return Math.max(info.mtimeMs, ...times, 0)
}

// The oldest asset rather than the newest: one build writes them together, so
// the oldest is that build's timestamp.
async function buildTime() {
  let entries
  try {
    entries = await readdir(DIST_ASSETS)
  } catch {
    return null
  }
  const assets = entries.filter((f) => f.endsWith('.css') || f.endsWith('.js'))
  if (assets.length === 0) return null
  const times = await Promise.all(
    assets.map(async (f) => (await stat(path.join(DIST_ASSETS, f))).mtimeMs),
  )
  return Math.min(...times)
}

/** Guarantee dist/assets reflects the current sources, building if it does not. */
export async function ensureFreshDist() {
  const built = await buildTime()
  const newestInput = Math.max(...(await Promise.all(INPUTS.map(newestMtime))))
  if (built !== null && built >= newestInput) return DIST_ASSETS

  console.log(
    built === null
      ? 'No build found in dist/assets. Building first: this check reads the built output.'
      : 'dist/assets is older than the sources that produced it. Rebuilding first.',
  )
  // shell: true because pnpm is pnpm.cmd on Windows.
  const build = spawnSync('pnpm build', { cwd: FRONTEND, stdio: 'inherit', shell: true })
  if (build.status !== 0) {
    console.error('\n`pnpm build` failed, so there is no current build to check against.')
    process.exit(1)
  }
  if ((await buildTime()) === null) {
    console.error('\n`pnpm build` succeeded but wrote no assets to dist/assets.')
    process.exit(1)
  }
  console.log()
  return DIST_ASSETS
}
