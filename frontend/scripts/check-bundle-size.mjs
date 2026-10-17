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
// 88 since the design pass (2026-10-04): the settings pages, the chapter's
// settings and the run report load on demand, so the launcher's first paint
// no longer carries them. Measured 87.4 KB.
// 88.1 since the page layer (2026-10-04): a page now slides over the chapter
// card inside the stage, so the layer that animates it, keeps focus and leaves
// the card inert (PageLayer, usePages) is in the launcher's own code and not in
// the lazy page chunks. The pages' own code, the page frame and the reveal
// stay in those chunks, and the three chapter pages now load as one. Measured
// 88.04 KB, from 87.8 KB before it.
// 88.3 since the mod switches (2026-10-04, issue 156): the bundled manifest,
// which the first paint embeds, names the mods a player may switch off for
// Frangfurd and Lichdenstein; the switches' own code loads with the chapter's
// settings. Measured 88.2 KB, from 88.04 KB before it.
// 88.6 since the hero waits for its pictures (2026-10-04): the bundled titles,
// icons and chapter pictures are decoded at startup, and a new backdrop drifts
// in only once it has decoded (lib/useLoadedImage). Both run from the first
// paint, so they cannot load on demand. Measured 88.5 KB alone, 88.6 KB with the
// wiki pictures setting beside it, from 88.2 KB before them.
// 89.2 since the account page (2026-10-05, issue 192): the sidebar's account
// tile is a button with a pressed state, and the page layer opens a second page
// of the launcher's own beside the settings, from the bottom, both from the
// first paint. The page itself loads on demand. Measured 89.1 KB, from 88.6 KB
// before it.
// 89.3 since Play offline and the Iris switch (2026-10-05, issues 192 and 156's
// follow-up): the sidebar's tile says Offline from the first paint, the settings
// carry the offline pair, and the mod store knows which switches are locked.
// Measured just over 89.2 KB, from 89.1 KB before them.
// 89.8 since the hero panorama (2026-10-05, issue 195): the hero's choice of picture
// (lib/heroArt), the panoramas the chapter store reads and the cube's lazy import
// run from the first paint. The cube itself loads only when the panorama is chosen.
// Measured 89.7 KB, from 89.3 KB before it.
// 90.1 since the panorama decodes before it draws (2026-10-05): the hero decodes a
// panorama's faces in the background and says it is loading meanwhile, so a chapter
// switch never waits on them (lib/panorama, the spinner glyph). Both run from the
// first paint. Measured 90.0 KB, from 89.7 KB before it.
// 90.4 since Reinstall (2026-10-11, issue 244): the action bar tells an instance the
// launcher did not set up from its own and offers Reinstall in Play's place, the
// engine store has the write behind it, and the corner says where the old instance
// went, with Open in Prism to it. All are the first paint's. Measured 90.34 KB, from
// 90.07 KB before it.
// 90.5 since the update notice (2026-10-14, ADR-9 amendment): the check and its
// popup load in a chunk of their own after the first paint, and what stays in the
// entry is the lazy import, its Suspense and the new binding with its model.
// Measured 90.45 KB, from 90.34 KB before it.
// 91.1 since the expandable columns (2026-10-17): each column under the actions
// opens a popup from the expand icon that shows on hover, and the wiki export
// carries a page's facts. The popups and their grow animation load in a chunk of
// their own; what stays in the entry is the column frame, the icon, the fact
// list's two-column layout, the lazy imports and the facts' model. Measured
// 91.04 KB, from 90.45 KB before it.
// 91.3 the same day: the dialog centres on the chapter card, closes from an X in
// its corner, and the popups' chunk is fetched on hover and shortly after the
// card shows. Measured 91.21 KB.
// 92.1 the same day: a column's popup opens on the click itself, so the dialog
// and the fade its body comes in with (Reveal) are the first paint's; only the
// bodies load on demand. Before, the whole popup waited for its chunk. Measured
// 92.06 KB locally and just over 92.1 KB on CI's runner, so 92.2.
// 92.4 the same day: a Play Prism ignores over the profile name ends in a popup of
// its own chunk, and the account tile shows the name the game reported for Prism's
// default account, read again once a game runs. What stays in the entry is the lazy
// import, the read, the name's fallback and the new reason. Measured 92.3 KB.
const BUDGET_KB = 92.4

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
