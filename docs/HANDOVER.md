# Handover

Written 2026-09-30, at the end of the second local session (Windows 11, the
author's PC), brought up to date the same day by a cloud session and again by
the third local session (Install, the local pack override, the first real
install). Read `agent_docs/ROADMAP.md` for the milestones, `docs/adr/` for
the decisions and the GitHub issues for the work items; this file is what a
fresh session cannot work out on its own.

## Picking up

```bash
git clone https://github.com/sandrogekeler/Kapital-Launcher
cd Kapital-Launcher/frontend && pnpm install && cd ../site && pnpm install && cd ..
wails dev                    # Wails CLI v2.16.0, the version go.mod names
.claude/suite-check.py       # all pass; aislop and release notes skip without ruff 0.16.7 or python3
```

`main` is the default branch. Branch from `origin/main`. The repository
deletes a branch when its pull request merges, which also retargets a pull
request stacked on it.

## Open pull requests

#37, #38 and #39 are merged. Merge in this order; each stacked one retargets
to `main` when the one below it merges.

| PR | What | State |
|---|---|---|
| #40 | Install replaces Play until the instance exists (#24) | Green |
| #42 | Install from a local `packwiz serve` (#41), stacked on #40 | Green; tested for real on the author's PC, results in its description |
| this one | The handover, stacked on #42 | |
| #48 | The logo's gradient colours as `color.brand` tokens | |
| #49 | Frangfurd shown as pack 1.0.0 with 96 mods | |
| kapital-packs#2 | The README's release steps without a `.mrpack` | |
| #1 | Dependabot: pnpm/action-setup 6.1.0 | Not mergeable, and not a stale base: on Windows 6.1.0 leaves `pnpm install` and `pnpm build` exiting 0 with no output, so `frontend/dist` is never built. `main` passed CI on the same base with 6.0.10. The version is skipped in `.github/dependabot.yml` (#51); #1 is closed |

## Work items, in the order agreed

Done this session: #22's writer (merged in #39) is called by Install (#40,
closes #24), and #42 (closes #41) lets it install from `packwiz serve`. #22
stays open until the author closes it.

**Frangfurd 1.0.0 is released** in `kapital-packs`: kapital-packs#1 merged
(new title video, Drippy loading screen and layout, FancyMenu's forced
fullscreen off, NeoForge's early window off), tagged `frangfurd-v1.0.0` with
a GitHub release and no `.mrpack` (it would embed the CurseForge jar). It is
not hosted yet, so players cannot install it: that is #25.

1. **#25** Publish the packs on Cloudflare Pages, add its `pages.dev` host to
   `AllowedManifestHosts`, test-install from `packwiz serve` first.
   Frangfurd already installs and launches end to end from `packwiz serve`
   (#42), so hosting is the remaining step for it; #35 comes first.
2. **#20** Launch the three real packs end to end, and **#5** the settings
   screen (its gear sits disabled in the header bar since #13). #5 also gets
   a field for `packOverrides`, set by hand in `settings.json` until then.
3. **#36** Memory and optional mods per player.
4. **#50** Install prepares the chapter (sign-in, Java, libraries, pack,
   assets happen at Install, not the first Play) with one progress view in
   the launcher; builds on #44 and #45. Filed after a clean new-player run on
   2026-09-30: Prism uninstalled (program only; its data folder with the
   sign-in and `kapital-lichdenstein` stays), the Frangfurd instance backed
   up to `D:\Private\Projects\Videogames\Minecraft\Projects\KapitelKapital\backups\kapital-frangfurd-2026-09-30`
   and removed, then Get Prism, Install and Play all worked. The managed
   Prism now lives in `%APPDATA%\KapitalLauncher\prism`.

Plan each with the author before building; they choose between the options.

Filed 2026-09-30 for work that needs the author, a dashboard or real hardware:
#29 (Pages build watch paths), #30 (macOS verification pass), #31 (universal
macOS build and ADR-7), #32 (two resource pack names with `§` and `⛈`), #33
(server addresses and join on launch), #34 (first Windows build and app icon),
#35 (what publishing the packs makes public).

## The packs

- **Source repository:** `sandrogekeler/kapital-packs`, private, cloned beside
  this one (`KapitalLauncher/kapital-packs`). One packwiz folder per chapter;
  only `frangfurd/` exists (released as 1.0.0, tag `frangfurd-v1.0.0`,
  NeoForge 21.1.252, Minecraft 1.21.1): 96 mods, 105 files referenced on
  Modrinth, Create Propulsion on CurseForge (`mode =
  "metadata:curseforge"`; it downloads with no manual step), plus its
  config, KubeJS, menu assets and resource pack.
- **`tools/import_instance.py`** regenerates a pack from a Prism instance. It
  matches jars and zips on Modrinth by hash, then CurseForge through `packwiz
  curseforge detect`, skips disabled mods, caches and per-player state, and
  marks server-only mods `both` because singleplayer needs them.
- **Files are stored byte for byte** (`.gitattributes`: `* -text`). The first
  commit normalised line endings and broke every `index.toml` hash; do not
  reintroduce `text=auto`.
- **No file over 25 MiB** (Cloudflare Pages). The Frangfurd title video
  (kapital-packs#1) is a two-pass 4K60 re-encode at a 40 Mbit/s target, which
  x264 settled at 31 Mbit/s, 11.7 MB. The master is
  `D:\Private\Projects\Videogames\Minecraft\Projects\KapitelKapital\assets\intro\frangfurd\Frangfurd-Intro.mp4`.
- **Frangfurd's fullscreen start** (the comment on #46): the title screen
  could stay in the bottom-left corner of a black screen, drawn at 854x480,
  the early window's size, because the game missed the resize to fullscreen.
  FancyMenu's `force_fullscreen` is off (kapital-packs#1); Minecraft's own
  `fullscreen:true` in the default options makes the game fullscreen.
  `earlyWindowMaximized = true` left the background shifted under a black band.
  **`earlyWindowControl = false` in `config/fml.toml` fixed it**, at the cost
  of a black window with a white rectangle, frozen for a moment before Drippy
  draws. The author wants players to see only the launcher's splash instead
  (#43, #45). Released in Frangfurd 1.0.0.
- **Not hosted yet.** The Pages project for `kapital-packs` does not exist;
  that is #25. Hosting makes every indexed file public, not only the
  metadata: #35.
- **Test-installed from `packwiz serve`** on 2026-09-30 (the comment on #25):
  529 of 529 files client and server side, Create Propulsion included with no
  manual step, a changed file refused by hash. Under a non-UTF-8 locale two
  resource packs fail to install (#32).
- `packwiz` is installed on the author's PC with `go install
  github.com/packwiz/packwiz@latest`.

## The author's PC

- **Prism 11.1.0** at `%LOCALAPPDATA%\Programs\PrismLauncher`, data root
  `%APPDATA%\PrismLauncher`. The launcher uses it; a managed Prism is only a
  fallback.
- **`kapital-frangfurd` is the instance Install wrote** from `packwiz serve`
  on 2026-09-30 (ZGC, 512 MB to 8 GB, Drippy loading screen), and the author
  chose it as the one they edit before the pack is uploaded; the importer in
  `kapital-packs` reads it. Its hand-copied predecessor, copied from the
  Modrinth profile "Morner", went to the Recycle Bin as
  `kapital-frangfurd-copied` (restorable from there).
- **Its packwiz sync is off while the author edits**: `OverrideCommands=false`
  in its `instance.cfg`, so Play starts the files as they are and nothing is
  reverted. The pre-launch command, syncing from
  `http://localhost:8080/pack.toml`, is still in the file, so the launcher
  still shows "Dev pack". The author's edits were imported and released as
  1.0.0; sync is still off. Setting `OverrideCommands=true` syncs again, and
  then needs `packwiz serve` running. Once #25 hosts the pack, the clean way
  is to delete the instance and Install from the hosted `pack.toml`.
- `%APPDATA%\KapitalLauncher\settings.json` carries
  `"packOverrides": {"frangfurd": "http://localhost:8080/pack.toml"}`. Remove it
  to go back to the manifest's pack (none is hosted yet, so Install is then
  disabled again).
- Default Options seeds `options.txt`, keybindings and the server list
  "Frangfurd (Global)" (`female-specified.gl.joinmc.link`) and "Frangfurd
  (Germany)" (`rails-enjoyed.tun.ply.gg`) on a first start.
- **Lichdenstein** has a `kapital-lichdenstein` instance in Prism now, made
  by the author. **Not in Prism yet:** Luxemburg (CurseForge instance, Forge
  43.5.0 on 1.19.2, 208 mods, 12 GB). Move it the same way: an empty Prism
  instance named `kapital-luxemburg` with the right loader, then copy the
  files in.
- **Never open the Modrinth App's `app.db`**: it holds account sign-ins.
- The Cloudflare Pages project for the download site builds every pull
  request unless its build watch paths are set (`site/README.md`). Ask
  whether the author set them.

## Open questions, all the author's

- **Server addresses and `joinOnLaunch`**: #33.
- **Memory** for Lichdenstein is `null` in the manifest. Frangfurd's is 8 GB
  (decided 2026-09-30), written as the instance's maximum with a 512 MB
  minimum.
- **`verdigris-bright`** in the wiki: two derived tints, not brand values.

## Things a fresh session will trip on

- **Git Bash heredocs collapse backslashes** in Python and TypeScript written
  through them. Write files with the editor tool, not `cat <<'EOF'`, when a
  backslash or an apostrophe inside quotes matters. It happened again this
  session: `\a` and `\f` in a Windows path became control characters.
- **Git Bash's `sed -i` strips CRLF.** Pack files are stored byte for byte
  and most of Frangfurd's configs have Windows line endings, so a one-line
  `sed` rewrites the whole file. Edit pack files in binary (Python, `rb`/`wb`)
  and check `git diff --stat` shows one line.
- **`gh pr edit --body-file` with an empty file wipes the description.** A
  Python step that fails on a `●` under the cp1252 console still leaves the
  file empty; write the body with the editor tool instead.
- **"The process cannot access the file" when renaming an instance folder:**
  look for another Claude Code session whose working directory is inside it
  (`list_sessions` shows each `cwd`), before blaming Explorer or Prism.
- **`wails dev` serves the app with live Go bindings at
  `http://localhost:34115`**, so the browser pane can drive the real backend
  (Install, Play) without the Wails window. Wails' own `ipc.js` logs one
  harmless `reading 'nodes'` error there.
- **`packwiz serve --refresh=false` reads `index.toml` once at start**:
  restart it after every `packwiz refresh`, or it serves the old index.
- **Symlinks:** this PC cannot create them without a privilege the Windows CI
  runner has, so a test can pass here and fail in CI. Check CI, not only the
  local run.
- **Prism facts** used by the managed Prism (the setup-wizard conditions, the
  updater's `prismlauncher_update.cfg`, `m_rootPath` being the program
  folder on Windows) were read from Prism 11.1.1's source and are cited in
  ADR-11. Re-read them when Prism's major version changes.
- **`[verify]` on a real Mac:** #30. The author's 2017 Intel iMac covers it;
  the bundle layout, its symlinks and the executable path were already
  checked against Prism 11.1.1's macOS zip.
- **The Wails CLI rewrites `go.mod` to its own version.** Install v2.16.0,
  and check `git status` after the first `wails dev`
  (`.claude/rules/builds-and-releases.md`).
- **pnpm's release-age policy** refuses packages younger than a day, in both
  `pnpm-workspace.yaml` files, on purpose (`.claude/rules/dependencies.md`).
- **aislop needs ruff 0.16.7 on PATH** and `release notes` needs `python3`, or
  the check runner skips them locally; CI runs both.
- **`wails build` has not been run**; `build/appicon.png` is the Wails
  template's placeholder (#34).
- **In a cloud container** every check runs, aislop included, with ruff
  0.16.7 from a venv on `PATH`. Java there follows a POSIX locale, so run
  packwiz-installer with `LC_ALL=C.UTF-8` (#32). The bootstrap's update check
  against `api.github.com` gets a 403 there; fetch `packwiz-installer.jar`
  from its release page and pass `--bootstrap-no-update`.
- **CodeQL and Scorecard are not vendored**: the repo is private.
- The vendored files (`.claude/suite-*.py`, three workflows, the notes
  generator, `.aislop/base.yml`) are copied from `kollektiv-mc/Kollektiv` by
  hand. Re-copy to update.
