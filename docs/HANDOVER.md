# Handover

Written 2026-09-30, at the end of the second local session (Windows 11, the
author's PC). Read `agent_docs/ROADMAP.md` for the milestones, `docs/adr/` for
the decisions and the GitHub issues for the work items; this file is what a
fresh session cannot work out on its own.

## Picking up

```bash
git clone https://github.com/sandrogekeler/Kapital-Launcher
cd Kapital-Launcher/frontend && pnpm install && cd ../site && pnpm install && cd ..
wails dev                    # Wails CLI v2.16.0, the version go.mod names
.claude/suite-check.py       # 23 checks; all pass except the two skipped locally
```

`main` is the default branch. Branch from `origin/main`. The repository
deletes a branch when its pull request merges, which also retargets a pull
request stacked on it.

## Open pull requests, and the order to merge them

| PR | Base | What | State at handover |
|---|---|---|---|
| #26 | `main` | Getting Prism, the Go side: `ManagedPrism`, verification, detection, ADR-11 | Review findings fixed; CI re-running after a Windows symlink fix |
| #27 | #26's branch | Getting Prism, the UI: approval card, progress, update offer | Stacked; merge after #26 and it retargets to `main` |
| #1 | `main` | Dependabot: pnpm/action-setup 6.1.0 | Labelled; `backend` fails until Dependabot rebases it (asked for) |

Merge #26 first, then #27. Both close #23.

## Work items, in the order agreed

1. **#22** Generate a Prism instance zip for a fresh install: memory from the
   chapter's `memoryGb`, a named JVM preset (e.g. `"jvm": "zgc"`, never raw
   arguments in the manifest), the packwiz-installer pre-launch command, the
   loader versions. Imported with `prismlauncher -I <local zip>`.
2. **#24** Install a chapter before it can be played: Install replaces Play
   when the `kapital-<id>` instance is missing. Nothing comes preinstalled.
3. **#25** Publish the packs on Cloudflare Pages, add its `pages.dev` host to
   `AllowedManifestHosts`, test-install from `packwiz serve` first.
4. **#20** Launch the three real packs end to end, and **#5** the settings
   screen (its gear sits disabled in the header bar since #13).

Plan each with the author before building; they choose between the options.

## The packs

- **Source repository:** `sandrogekeler/kapital-packs`, private, cloned beside
  this one (`KapitalLauncher/kapital-packs`). One packwiz folder per chapter;
  only `frangfurd/` exists (version 1.0.0, NeoForge 21.1.252, Minecraft
  1.21.1): 104 files referenced on Modrinth, Create Propulsion on CurseForge
  (`mode = "metadata:curseforge"`; whether it downloads without a manual step
  is untested), plus its config, KubeJS, menu assets and resource pack.
- **`tools/import_instance.py`** regenerates a pack from a Prism instance. It
  matches jars and zips on Modrinth by hash, then CurseForge through `packwiz
  curseforge detect`, skips disabled mods, caches and per-player state, and
  marks server-only mods `both` because singleplayer needs them.
- **Files are stored byte for byte** (`.gitattributes`: `* -text`). The first
  commit normalised line endings and broke every `index.toml` hash; do not
  reintroduce `text=auto`.
- **No file over 25 MiB** (Cloudflare Pages). The Frangfurd title video is a
  40 Mbit/s 4K re-encode (16.8 MB); the 68 MB original stays in the author's
  Modrinth profile.
- **Not hosted yet.** The Pages project for `kapital-packs` does not exist;
  that is #25.
- `packwiz` is installed on the author's PC with `go install
  github.com/packwiz/packwiz@latest`.

## The author's PC

- **Prism 11.1.0** at `%LOCALAPPDATA%\Programs\PrismLauncher`, data root
  `%APPDATA%\PrismLauncher`. The launcher uses it; a managed Prism is only a
  fallback.
- **`kapital-frangfurd`** Prism instance: copied from the Modrinth profile
  "Morner" (the Modrinth App refuses to export it: "loader mismatch"). JVM
  arguments `-XX:+UseZGC -XX:+ZGenerational` (Distant Horizons warns under
  G1), memory 3 to 8 GB, Default Options seeds `options.txt`, keybindings and
  the server list "Frangfurd (Global)" (`female-specified.gl.joinmc.link`) and
  "Frangfurd (Germany)" (`rails-enjoyed.tun.ply.gg`).
- **Not in Prism yet:** Lichdenstein (Modrinth App profile, Fabric 0.16.10 on
  1.20.6, 31 mods) and Luxemburg (CurseForge instance, Forge 43.5.0 on 1.19.2,
  208 mods, 12 GB). Move them the same way: an empty Prism instance named
  `kapital-<chapter>` with the right loader, then copy the files in.
- **Never open the Modrinth App's `app.db`**: it holds account sign-ins.
- The Cloudflare Pages project for the download site builds every pull
  request unless its build watch paths are set (`site/README.md`). Ask
  whether the author set them.

## Open questions, all the author's

- **Lichdenstein's server address**: `placeholder.invalid` until it has a
  permanent one. Frangfurd's manifest address is the Germany tunnel; the
  Global one is `female-specified.gl.joinmc.link`. Which the status line
  should ping is the author's call.
- **Memory** for Lichdenstein and Frangfurd is `null` in the manifest;
  Frangfurd runs with 8 GB in Prism. Both feed #22.
- **Frangfurd `joinOnLaunch`** is `false`.
- **`verdigris-bright`** in the wiki: two derived tints, not brand values.

## Things a fresh session will trip on

- **Git Bash heredocs collapse backslashes** in Python and TypeScript written
  through them. Write files with the editor tool, not `cat <<'EOF'`, when a
  backslash or an apostrophe inside quotes matters.
- **Symlinks:** this PC cannot create them without a privilege the Windows CI
  runner has, so a test can pass here and fail in CI. Check CI, not only the
  local run.
- **Prism facts** used by the managed Prism (the setup-wizard conditions, the
  updater's `prismlauncher_update.cfg`, `m_rootPath` being the program
  folder on Windows) were read from Prism 11.1.1's source and are cited in
  ADR-11. Re-read them when Prism's major version changes.
- **`[verify]` on a real Mac:** the managed Prism's `codesign` check, its app
  bundle's symlinks, the Sparkle updater setting, the macOS Prism paths, the
  header bar's traffic lights (ADR-10).
- **The Wails CLI rewrites `go.mod` to its own version.** Install v2.16.0,
  and check `git status` after the first `wails dev`
  (`.claude/rules/builds-and-releases.md`).
- **pnpm's release-age policy** refuses packages younger than a day, in both
  `pnpm-workspace.yaml` files, on purpose (`.claude/rules/dependencies.md`).
- **aislop needs ruff 0.16.7 on PATH** and `release notes` needs `python3`, or
  the check runner skips them locally; CI runs both.
- **`wails build` has not been run**; `build/appicon.png` is the Wails
  template's placeholder.
- **CodeQL and Scorecard are not vendored**: the repo is private.
- The vendored files (`.claude/suite-*.py`, three workflows, the notes
  generator, `.aislop/base.yml`) are copied from `kollektiv-mc/Kollektiv` by
  hand. Re-copy to update.
