# Handover

Written 2026-09-30 over four local sessions and a cloud one, rewritten
2026-10-01 at the end of the fifth local session (Windows 11, the author's
PC): the game tracker (#44), five polish items (#83 to #87), and hiding the
game window (#45, ADR-0012). Read `agent_docs/ROADMAP.md` for the milestones,
`docs/adr/` for the decisions and the GitHub issues for the work items; this
file is what a fresh session cannot work out on its own.

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

Everything up to #92 is merged and `main` is green (CI, CodeQL, Scorecard,
aislop).

| PR | What | Note |
|---|---|---|
| #93 | Keep the game window hidden until the resource reload (#45, ADR-0012) | Needs kapital-packs#3 |
| this one | The handover after the fifth session | |
| `claude/keen-meitner-kuompl` | The download page shows the launcher window cycling through the chapters (cloud session, 2026-10-01) | Checked in headless Chromium only, not in a real browser |
| kapital-packs#3 | Frangfurd's NeoForge early window back on | The pack version stays 1.0.0; a 1.0.1 is the author's call |

## Done on 2026-10-01

- **#44, merged (#82).** `GameTracker` follows a launched start: a fresh
  `latest.log` (judged against a snapshot taken before Play), marker lines
  only, the game's Java found as a child of Prism (Toolhelp on Windows,
  `kern.proc` on macOS) and waited on. Phases go out as `game:state`; the
  state line shows them, Play and Install wait. Timings of the last five
  starts per chapter are in `launchtimes.json` for #43's estimate. Verified
  with a clean close (exit 0) and a killed Java (`crashed`, exit -1).
- **#83 to #87, merged (#88 to #92):** frontend errors in the Go log, Copy log
  (redacted, checked on the real log: no name, path or address), Open folder
  from a chapter's pen, CodeQL and Scorecard (vendored from Konnekt, not
  Kollektiv), the About section with the licences. All checked in the real
  app.
- **#45, in #93.** Hiding the game window from outside lands only when the
  game's thread handles messages. With `earlyWindowControl = false` it does
  not during mod loading, and the window stayed visible for 19 s; with the
  early window on, hides landed in 0 to 31 ms and the window was never seen.
  The handover is the resource reload beginning, so Drippy is seen; there a
  fullscreen window is made one pixel shorter and restored, which fixes the
  corner glitch #46 was about (3 of 4 starts had it). The author saw Drippy,
  the intro and the title at full size. Behind the developer setting
  `holdGameWindow` until #43 covers the hidden time.

## Work items, in the order agreed

1. **#43** The loading splash, now that #44 and #45 exist. Decided: launcher
   starts only, minimise once the game has the window and come back when the
   tracker sees it end. Open: what the splash does about Prism's "Please
   wait..." dialogs and packwiz-installer's window, which show before the
   game (#45's findings comment lists them).
2. **#25** Publish the packs on Cloudflare Pages; #35 first (what becomes
   public is the author's call).
3. **#20** Launch the three real packs end to end. Luxemburg is not in Prism,
   and `kapital-lichdenstein` sits in `%APPDATA%\PrismLauncher`, the root the
   launcher no longer looks at. Lichdenstein is Fabric 1.20.6 and has never
   run in Prism, so its log markers and #45's early-window behaviour are
   unmeasured.
4. **#36**, second half: mod toggles, once `kapital-packs` marks mods optional.
5. **#50** Install prepares the chapter with one progress view. The
   pre-launch route cannot work: Prism runs the pre-launch command before it
   downloads libraries and assets (`MinecraftInstance.cpp`,
   `createLaunchTask`, 11.1.1). What is left is letting the game start and
   closing it on a #44 marker. `prismlauncher.cfg`'s console keys are
   `ShowConsole` (default false), `AutoCloseConsole`, `ShowConsoleOnError`
   (default true; after a crash Prism stays open with its console),
   `QuitAfterGameStop` (true in the managed root).

Plan each with the author before building; they choose between the options.
Building is done by Sonnet agents from a written brief, one issue per pull
request; parallel ones that each add a bound method conflict on `app.go` and
the generated bindings, so stack them before handing over.

Filed 2026-09-30 for work that needs the author, a dashboard or real hardware:
#29 (Pages build watch paths), #30 (macOS verification pass, now including
the tracker's and the window holder's macOS paths), #31 (universal macOS build
and ADR-7), #32 (two resource pack names with `§` and `⛈`), #33 (server
addresses and join on launch), #34 (first Windows build), #35 (what
publishing the packs makes public).

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
- **Frangfurd's fullscreen start** (#46): with NeoForge's early window on,
  the title screen could stay in the bottom-left corner of a black screen,
  drawn at 854x480, because the game missed the resize to fullscreen.
  Frangfurd 1.0.0 turned the early window off (`earlyWindowControl = false`),
  which #45 cannot hide in time. kapital-packs#3 turns it back on; the
  launcher fixes the glitch with its one-pixel nudge at the handover (#93). A
  start from Prism directly still gets the glitch now and then.
- **This repository is public now**, so everything in it is: the manifest
  with the tunnel addresses, the art, the checklists. Nothing secret was in
  it (S1.2). `kapital-packs` and the wiki are still private.
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

- **Prism's own install is gone** since the clean run: the launcher uses the
  managed Prism 11.1.1 in `%APPDATA%\KapitalLauncher\prism` (`root/` is its
  data root, `kapital-frangfurd` in it). `%APPDATA%\PrismLauncher` still
  holds the old data with `kapital-lichdenstein`, which the launcher no
  longer sees.
- **`kapital-frangfurd` is the instance Install wrote** from `packwiz serve`
  on 2026-09-30 (ZGC, 512 MB to 8 GB, Drippy loading screen), and the author
  chose it as the one they edit before the pack is uploaded; the importer in
  `kapital-packs` reads it. Its hand-copied predecessor, copied from the
  Modrinth profile "Morner", went to the Recycle Bin as
  `kapital-frangfurd-copied` (restorable from there).
- **Its packwiz sync is on again** (`OverrideCommands=true`), syncing from
  `http://localhost:8080/pack.toml`, so Play needs `packwiz serve` running in
  `kapital-packs/frangfurd`. On 2026-10-01 its `config/fml.toml` matched the
  pack's hash (early window off) after the #45 tests, which ran with sync off
  and were restored from backups.
- `%APPDATA%\KapitalLauncher\settings.json` carries
  `"packOverrides": {"frangfurd": "http://localhost:8080/pack.toml"}`. Clear
  the field under Developer on the settings screen (#54) to go back to the
  manifest's pack (none is hosted yet, so Install is then disabled again).
  It also carries `"holdGameWindow": true` from the #45 tests: on a build
  with #93 the game window stays hidden until the reload (fine with the pack's
  early window on, a late hide with it off). Untick it under Developer.
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

- **aislop does not run on this PC** (no ruff 0.16.7 on `PATH`), so the local
  suite skips it and CI is the first to judge. #93 failed it on a Win32
  callback's seven parameters; a fixed signature gets an inline
  `aislop-ignore-next-line <rule> -- <reason>`.
- **Settings written behind the store's back are lost**: the frontend's
  settings store saves its own copy (on a chapter switch, for one), so a field
  set through `SaveSettings` from the console disappears. Change settings
  through the settings screen.
- **Following a real start from outside**: `Get-CimInstance Win32_Process`
  gives parent ids (launcher, then `prismlauncher.exe`, then `javaw.exe`); a
  PowerShell `EnumWindows` probe every 100 ms shows every window of those
  processes; `PrintWindow` with flag 2 captures the game's window even behind
  others. Scripts that did this on 2026-10-01 are described in #45's
  comments. The game's window is `GLFW30`; Prism's progress dialogs are
  `Qt6102QWindowIcon`; packwiz-installer's is `SunAwtFrame`.
- **`kapital-packs` needs long paths**: a worktree under the scratchpad fails
  with "Filename too long" in the resource pack. Put worktrees beside the repo
  (`KapitalLauncher/kp-early` held kapital-packs#3) and set
  `core.longpaths true`. Removing a launcher worktree with `node_modules` needs
  PowerShell's `Remove-Item -LiteralPath "\\?\<path>"`.
- **Testing a pack setting on the instance**: Play's packwiz sync puts every
  indexed file back, so switch `OverrideCommands` off for the test, back up
  `instance.cfg` and the file, and restore both after.

- **`.claude/suite-check.py` needs `PYTHONUTF8=1`** on this PC once any
  test prints a non-cp1252 character (a `✓`, an em dash in a diff): the
  vendored runner decodes subprocess output as cp1252 and dies with
  `UnicodeDecodeError` instead of reporting the failing check.
- **`wails dev`'s Go watcher did not rebuild** after edits to `backend/`
  twice on 2026-09-30; the frontend reloaded, the bindings did not. Restart
  it. A `wails dev` from an earlier session may still hold port 34115
  (`Get-CimInstance Win32_Process` shows `wails dev` and
  `kapital-launcher-dev.exe`); the `wails-dev` preset in
  `.claude/launch.json` starts a fresh one with live bindings.
- **jsdom has no `AnimationEvent`**, so React listens for
  `webkitAnimationEnd` there: a test that must end a CSS animation
  dispatches that name (`ChapterStage.test.tsx`, `App.test.tsx`'s
  `switchTo`), or the leaving card stays mounted.
- **Vite keeps a stale `wailsjs/go/main/App.js`** after `wails generate
  module` adds a method; restart the frontend dev server or the page fails
  with "does not provide an export".
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
- **`wails build` has not been run** (#34); `build/appicon.png` is the
  Kapitel Kapital mark since #79.
- **In a cloud container** every check runs, aislop included, with ruff
  0.16.7 from a venv on `PATH`. Java there follows a POSIX locale, so run
  packwiz-installer with `LC_ALL=C.UTF-8` (#32). The bootstrap's update check
  against `api.github.com` gets a 403 there; fetch `packwiz-installer.jar`
  from its release page and pass `--bootstrap-no-update`.
- **The repository is public since 2026-09-30 evening**, made so because the
  private allowance of Actions minutes ran out mid-stack (`backend-macos`
  costs ten Linux minutes a run) and every job failed unstarted. Actions on
  standard runners is free now, and CodeQL and Scorecard run (#88).
- The vendored files (`.claude/suite-*.py`, three workflows, the notes
  generator, `.aislop/base.yml`) are copied from `kollektiv-mc/Kollektiv` by
  hand, and `codeql.yml` and `scorecard.yml` from `kollektiv-mc/Konnekt`.
  Re-copy to update.
