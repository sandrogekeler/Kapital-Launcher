# Handover

Written 2026-09-28, at the end of the cloud session that built the scaffold.
Everything below is true of the branch `claude/nice-lovelace-s3rybr` at its
last push. Read `agent_docs/ROADMAP.md` for the milestones and `docs/adr/` for
the decisions; this file is what a fresh session cannot work out on its own.

## Picking up

```bash
git clone https://github.com/sandrogekeler/Kapital-Launcher
cd Kapital-Launcher/frontend && pnpm install && cd ..
wails dev                    # Wails CLI v2.16.0, the version go.mod names
.claude/suite-check.py       # 21 checks; all pass at the last push
```

pnpm 11 or later, Node 22 or later, Go 1.26 or later (a newer Go is fine;
`go.mod` pins the minimum). `wails doctor` was green on the author's Windows
11 machine and `wails dev` opened the window.

## Where things stand

| Repo | Branch | State |
|---|---|---|
| `Kapital-Launcher` | `claude/nice-lovelace-s3rybr` | 5 commits. **No `main` exists yet.** Create it at the scaffold commit, `git push origin 07f4658:refs/heads/main`, set it as the default branch, then open the pull request from the work branch. |
| `kapitel-kapital-wiki` | `claude/nice-lovelace-s3rybr` | [PR #13](https://github.com/sandrogekeler/kapitel-kapital-wiki/pull/13): the era accent correction. Cloudflare preview built; waiting on review. |

Dependabot already opened a branch on the launcher bumping a pinned action.
It will fail `pr-labelled` until the labels below exist and it is labelled.

## First things in the next session

1. **Create the labels.** `.github/labels.yml` declares them; nothing syncs
   it. With `gh` signed in: `python3 scripts/sync-labels.py`. Until then the
   issue forms reference labels GitHub silently ignores and every pull
   request fails the label gate.
2. **File the issues** in the table below, three labels each. Work items are
   issues; the roadmap is direction only (`agent_docs/CLAUDE.md`).
3. **Record the two observations from the first real run** in the issues
   for milestone 2: what the engine card said, and where `prismlauncher.exe`
   lives if it said "not found". The paths in `backend/services/prism.go` are
   marked `[verify]` because they come from the installers' defaults, not
   from an install.

## Issues to file

| Title | type | area | p | Milestone |
|---|---|---|---|---|
| Prism install paths on Windows 11 and macOS | chore | engine | p1 | 2 |
| Prism version read on a Windows GUI build | chore | engine | p2 | 2 |
| Settings screen | feature | engine | p1 | 2 |
| Instance detection under the Prism root | feature | engine | p1 | 2 |
| Launch each chapter end to end | chore | engine | p1 | 3 |
| Behaviour when Prism is already running | chore | engine | p2 | 3 |
| Open instance folder | feature | chapters | p3 | 3 |
| Frangfurd pack in packwiz | feature | packs | p1 | 4 |
| Fresh install through Prism import | feature | packs | p1 | 4 |
| Pack sync through a pre-launch template | feature | packs | p1 | 4 |
| Download verification by hash | feature | packs | p1 | 4 |
| Developer override for a local packwiz serve | feature | packs | p2 | 4 |
| Update pack button and sync state line | feature | chapters | p2 | 4 |
| launcher.json endpoint on the site | feature | chapters | p2 | 5 |
| Manifest fetch with cache and offline fallback | feature | chapters | p2 | 5 |
| Screenshots by URL from the site | feature | chapters | p3 | 5 |
| Real server addresses in the manifest | chore | server | p1 | 6 |
| MOTD and player sample in the UI | feature | server | p3 | 6 |
| Changelog panel fed by the manifest | feature | chapters | p3 | 7 |
| Copy redacted log action | feature | chapters | p2 | 7 |
| About screen with disclaimer and licences | feature | chapters | p2 | 7 |
| Forward render errors to the Go log | chore | build | p3 | 7 |
| Release workflow for Windows and macOS | feature | build | p2 | 7 |
| App icon | chore | build | p3 | 7 |

Milestones map to `agent_docs/ROADMAP.md`. Stage them as `milestone:` labels
or GitHub milestones, whichever the next session prefers; nothing reads them.

## Open questions, all the author's

- **Lichdenstein's client loader.** The visuals pack's loader is
  `[PLACEHOLDER]` in `data/launcher.json` (Fabric with Iris? Something else?).
- **Server addresses** for Lichdenstein and Frangfurd: `placeholder.invalid`
  until known. Everything reads as offline until then.
- **Frangfurd `joinOnLaunch`.** Set to `false` (play the pack, join from
  inside the game). One boolean if that is wrong.
- **`verdigris-bright`** in the wiki: two derived tints, not brand values.
  Flagged on PR #13.
- **The packwiz repository**: where it lives and where `pack.toml` is hosted
  (ADR-3). Nothing in milestone 4 can be tested end to end before one pack
  exists.

## Things a fresh session will trip on

- **The Wails CLI rewrites `go.mod` to its own version.** Install v2.16.0,
  and check `git status` after the first `wails dev`
  (`.claude/rules/builds-and-releases.md`).
- **pnpm's release-age policy** refuses packages younger than a day. It is
  set in both `pnpm-workspace.yaml` files on purpose; wait rather than relax
  it (`.claude/rules/dependencies.md`).
- **aislop needs ruff 0.16.7 on PATH** or the check runner skips it, never
  passes it. A venv is fine (`.claude/rules/aislop.md`).
- **`wails build` was never run.** The container had no WebView, so the
  Windows binary was cross-compiled with plain `go build` and `wails dev`
  was first run by the author. `build/appicon.png` is the Wails template's
  placeholder.
- **CodeQL and Scorecard are not vendored**: the repo is private. Add them
  when it is not (`agent_docs/SECURITY_CHECKLIST.md`, S8).
- The vendored files (`.claude/suite-*.py`, three workflows, the notes
  generator, `.aislop/base.yml`) are copied from `kollektiv-mc/Kollektiv` by
  hand, since this repo is not in that suite's manifest. Re-copy to update.
