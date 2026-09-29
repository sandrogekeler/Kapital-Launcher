# Kapital Launcher: Project Health Checklist

An evergreen yardstick across four pillars: **Clean, Stable, Scalable,
Performant.** Compare the tree against it before each milestone. Do not edit an
item to match what the code does; it is the target. A gap goes under `Open
backlog`, gets fixed, and the list is re-run.

The mechanical half is `.claude/suite.json`, run by `.claude/suite-check.py`
and by CI. Items below that a gate already holds say so; the rest are review.

## 1. Clean

- **Every colour and size is a token.** `design/tokens.json` is the only place a
  value is defined; components use `bg-canvas`, `text-fg-muted`, `text-sm` and
  so on. Gate: the `no literal colours` and `no arbitrary pixel text sizes`
  invariants, `pnpm check-tokens`.
- **Generated files are regenerated, never edited.** `tokens.css`, `tokens.ts`,
  `design_gen.go`, `wailsjs/`. Gate: the `generated` section.
- **Icons come from one module.** Gate: ESLint and the `icons come from one
  module` invariant.
- **No inline `style`.** Gate: ESLint `no-restricted-syntax`.
- **Each fact lives in one file.** `CLAUDE.md` links, it does not restate. The
  always-loaded memory (root `CLAUDE.md` plus `agent_docs/CLAUDE.md`) stays
  under 200 lines. Gate: the runner's `memory` section.
- **Published copy carries no em dash.** Gate: the `no em dashes` invariant over
  `frontend/src`, `data/`, `README.md` and `wails.json`; review for commits and
  pull requests.

## 2. Stable

- **Every bound method returns an error.** Gate: `TestBoundMethodsReturnAnError`.
- **Every value that reaches Prism is validated.** `LaunchArgs` refuses anything
  that is not a plain instance id, `host[:port]`, a profile name or an absolute
  root; the manifest is refused whole on a bad URL or an unknown chapter. Gate:
  `prism_test.go`, `manifest_test.go`.
- **A rejection is handled where the data lives.** Stores record `error`, writes
  revert, reads degrade to the bundled manifest or "unknown". Gate: the store
  tests.
- **Settings are written atomically and owner-only.** Gate: `settings_test.go`.
- **Coverage floors are ratchets.** `vite.config.ts`'s threshold rises with
  coverage and is never lowered to make a build pass. A Go floor arrives with
  the first service large enough to need one.

## 3. Scalable

- **A chapter is data.** Adding one is an entry in `data/launcher.json` and an
  accent in `design/tokens.json`; no component names a chapter.
- **The manifest shape is versioned** and refused when unknown, so the site can
  publish it later without the app guessing.
- **Dependencies are recorded** in `DEPENDENCIES.md` with a rationale, and
  Dependabot keeps them current.
- **A new store is a new domain.** Chapters, engine and settings do not read
  each other's state; App composes them.

## 4. Performant

- **Entry chunk budget.** Gate: `pnpm check-bundle`. Fonts and screenshots are
  separate assets and do not count.
- **Detection and launch never block the UI.** `Detect` runs on startup with a
  timeout on the version read; `Launch` returns once Prism has started.
- **No polling.** Engine state is read on start and on demand; instances also
  on window focus, since they appear when the user imports one in Prism.

## Open backlog

- Coverage floor for `backend/services` once it has a floor-sized surface.
- `[verify]` markers in `backend/services/prism.go`: the macOS and Linux install
  paths and their `--version` output. Windows was observed on 2026-09-29
  (Roadmap, milestone 2).
- Forward frontend render errors to the Go log (milestone 7).
