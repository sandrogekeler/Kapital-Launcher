# Handover

Written 2026-09-30 over four local sessions and a cloud one, rewritten
2026-10-01 after the fifth local session, and again 2026-10-02 at the end of
a cloud session that opened six pull requests (#105 to #110) from the state
after #99 to #104 merged. Read `agent_docs/ROADMAP.md` for the milestones,
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

## The pull requests of 2026-10-02

The six below were opened from one cloud session; each is one concern, built
by a Sonnet agent from a written brief and reviewed by the session before it
was opened. #105 to #109 merged the same day and `main` is green (CI, CodeQL,
Scorecard, aislop, Build); #110 followed after a base merge over #106 and
#109. None has been run on real hardware.

| PR | What | Verified | Still to see |
|---|---|---|---|
| #105 | The download page's GitHub button | `pnpm build` in `site/` | The page on Cloudflare |
| #106 | #103: a start ends the moment Prism's launch fails, with the reason on the card | Unit tests on a synthetic Prism log fixture | A real `logs/PrismLauncher-0.log` (the author's PC has one from 2026-10-02): the `LaunchTask(...) failed:` line and QDebug's quoting `[verify]` |
| #107 | `build.yml`: `wails build` on Windows and macOS for every pull request, releases cut from the Actions tab | Its own Build run: both jobs green in 3 minutes, `lipo` shows x86_64 and arm64, artefacts uploaded | The first `v0.1.0-alpha.1` dispatch (attestation, `checksums.txt`, the notes generator on a first release, `gh release create`) |
| #108 | The zip extractor goes through `os.Root`, so a symlink chain cannot escape | Tests with crafted zips, the bundle fixture with its 48 links | Nothing hardware-bound |
| #109 | Quitting while the loading card opens or closes no longer stalls | Race tests with a parked fake host | A quit during a start on Windows and on the iMac |
| #110 | macOS: card data store, show after un-minimise, full screen, App Nap, WebContent reload, autorelease pools | Apple docs and Wails' `.m` files read for every symbol; `backend-macos` is the first compile | Everything on the iMac (#30) |

`kapital-packs#3` (Frangfurd's NeoForge early window back on) is still the
pack's open item; the pack version stays 1.0.0 and a 1.0.1 is the author's
call.

### The second round, the same day

After #106 merged the author tried a start with `packwiz serve` down and the
card still waited: Prism's own `qtlogging.ini` silences the `launcher.task`
category, Critical included, so the line #106 waits for is never written by a
stock Prism. The same session then built, from the author's three asks and
two aesthetic fixes, five more pull requests, all merged the same afternoon,
and a sixth for the test that turned main red after the last of them:

| PR | What | Verified | Still to see on the PC |
|---|---|---|---|
| #114 | The managed root gets a copy of Prism's `qtlogging.ini` plus `launcher.task.critical=true`, so #106's line is written | The author: a failed sync now ends the start at once | The first Play of a Prism updated past 11.1.1 (the copy is of the installed version's file) |
| #113 | Server status back on the right of the action bar, the game's status beside Play | The author, on the real bar | |
| #115 | Play becomes Stop during a run (one click while starting, "Stop the game?" once a world may be up); `GameTracker.Stop` ends only the game's Java or the launcher's Prism, by pid | Rig tests with fake processes | `TerminateProcess`, `WM_CLOSE`, Prism exiting 0 on it; `SIGTERM` on the iMac |
| #116 | The run report: reason, timeline, redacted end of the game log and crash report name, on the card and behind Details beside Play; Copy log reports on the button; failure lines are one sentence (ADR-2 sixth amendment) | Tests on a redaction fixture; the bundle is at 89.6 of 90 KB gzip | The card's layout at 405 px with a real crash; a real NeoForge crash log through the redactor |
| #117 | Prism's console hidden on a failure (Windows, splash on), kept, shown from the report on request; its appearance is a second failure signal; the Prism is closed at the next Play, Stop or quit | Rig tests with a fake holder; `GOOS=windows go vet` | The title prefix `Console window for`, the show and foreground, `WM_CLOSE` to a hidden window; the real-window test in `gamewindow_console_windows_test.go` |
| #119 | The console hold's sweep test waits on the hold's own count: it read the count a moment early under coverage instrumentation on the Windows runner, which left main red after #117 (`go test` green, `coverage-floor` red, the same test) | `GOOS=windows go vet`; the Windows `backend` job on the pull request is the proof | |

Known limit from #117 (in ADR-12): a player's own Prism with `ShowConsole=true`
and the splash on reads as a failed start, because its console appears during
`starting`. The managed root seeds it off.

## Done on 2026-10-01 and 2026-10-02

- **#97, merged (#102).** The loading card is a borderless window of its own
  on Windows (Win32 popup, second WebView2) and macOS (`NSWindow`,
  `WKWebView`, cgo); the launcher minimises while the card is up. Verified on
  the author's PC; macOS not compiled locally then.
- **#25, merged (#101).** Frangfurd's pack is served by a Cloudflare Worker at
  `kapital-packs.alessandrogekeler.workers.dev` (what is public was decided on
  #35); Install works without a local `packwiz serve`. A full headless install
  from the hosted URL ran clean.
- **The download page (#99, #104)** shows the launcher window cycling through
  the chapters on CSS alone; the heading is the one-line logo.
- **#100, merged.** The tracker tests wait for the run and its last event, so
  `backend-macos` stops failing intermittently.
- **2026-10-02, the cloud session:** the six pull requests above, the roadmap
  ticks for #83 to #85, #87, the fresh install, the sync, the hash check, the
  Pages project, the GitHub link and the release workflow, and this handover.
  Facts behind #106 and #107 were read from Prism 11.1.1's source, Wails
  v2.16.0's docs and source, the runner image readmes and the actions'
  repositories, not from memory; the pull request bodies cite them.

## Work items, in the order agreed

1. **Run the first prerelease** from the
   Actions tab (Build, "Run workflow", `v0.1.0-alpha.1`). That is the test of
   the release half of #107, and it gives the iMac a universal bundle for #30
   without building there. Then set `site/links.json`'s `download`.
2. **#30 on the iMac**, now with more to check than the list on the issue:
   the card on macOS (opt-in under settings until then), #109's quit during a
   start, #110's four behaviours, and Gatekeeper's prompt on the unsigned
   bundle (System Settings, Privacy & Security, Open Anyway).
3. **#50** Install prepares the chapter with one progress view. The
   pre-launch route cannot work: Prism runs the pre-launch command before it
   downloads libraries and assets (`MinecraftInstance.cpp`,
   `createLaunchTask`, 11.1.1, confirmed again 2026-10-02: the order is
   CreateGameFolders, load meta, AutoInstallJava, CheckJava, VerifyJavaInstall,
   PreLaunchCommand, ClaimAccount, the update tasks, then the game). What is
   left is letting the game start and closing it on a #44 marker, or the
   progress view following Prism's launcher log, which #106 now reads for one
   marker and could read for the stages (`Task "..." failed:` lines are the
   only launch text that reaches `PrismLauncher-0.log`; the step names do not,
   so stage markers would come from the game log and the Java download's own
   lines `[verify]`). `prismlauncher.cfg`'s console keys are `ShowConsole`
   (default false), `AutoCloseConsole` (false), `ShowConsoleOnError` (true),
   `QuitAfterGameStop` (true in the managed root); an instance overrides them
   only with `OverrideConsole=true`.
4. **#20** Launch the three real packs end to end. Luxemburg is not in Prism,
   and `kapital-lichdenstein` sits in `%APPDATA%\PrismLauncher`, the root the
   launcher no longer looks at. Lichdenstein is Fabric 1.20.6 and has never
   run in Prism, so its log markers and #45's early-window behaviour are
   unmeasured.
5. **#36**, second half: mod toggles, once `kapital-packs` marks mods optional.

Plan each with the author before building; they choose between the options.
Building is done by Sonnet agents from a written brief, one issue per pull
request; parallel ones that each add a bound method conflict on `app.go` and
the generated bindings, so stack them before handing over. In a cloud
container, run at most two agents at a time: five in parallel, each
installing `node_modules` and running the suite, restarted the container
twice on 2026-10-02 and lost their uncommitted work until the worktrees were
resumed.

Filed 2026-09-30 for work that needs the author, a dashboard or real hardware:
#29 (Pages build watch paths), #30 (macOS verification pass), #31 (universal
macOS build and ADR-7; #107 builds `darwin/universal` already, so the ADR is
behind the workflow), #32 (two resource pack names with `§` and `⛈`), #33
(server addresses and join on launch), #34 (first Windows build: #107 built
one, the icon check on the PC is what is left), #35 (decided on 2026-10-01).

## Open questions, all the author's

- **The bundle identifier.** `build/darwin/Info.plist` still carries the
  template's `com.wails.{{safeBundleID .Name}}`. It is the app's identity on
  macOS (preferences, WebKit data), so changing it later changes identity;
  decide before the first release.
- **ADR-7 and Intel Macs.** #107 builds a universal bundle because the only
  Mac to test on is an Intel iMac. ADR-7 still says Apple Silicon only (#31).
- **The `codesign` check's strength.** `verify_darwin.go` runs
  `codesign --verify --deep --strict`, which an ad-hoc signature passes.
  Prism's releases are Developer ID signed; a `-R` requirement with Prism's
  Team ID (from `codesign -dv` on the iMac, `[verify]`) would check who signed.
- **macOS 14 and later activation** is cooperative: when the card closes at
  the game's window, the game's own activation request can be refused while
  the launcher stays active. The fix is `yieldActivationToApplication:` with
  the game's pid at the handover; the card does not know the pid yet. Not
  reproducible on the iMac (macOS 13).
- **Server addresses and `joinOnLaunch`**: #33.
- **Memory** for Lichdenstein is `null` in the manifest. Frangfurd's is 8 GB
  (decided 2026-09-30), written as the instance's maximum with a 512 MB
  minimum.
- **`verdigris-bright`** in the wiki: two derived tints, not brand values.
- **#22** is built (memory, the JVM preset, the pinned jars) and seen end to
  end; closing it, and updating ADR-3's status line, is a decision to record.

## The packs

- **Source repository:** `sandrogekeler/kapital-packs`, private, cloned beside
  this one (`KapitalLauncher/kapital-packs`). One packwiz folder per chapter;
  only `frangfurd/` exists (released as 1.0.0, tag `frangfurd-v1.0.0`,
  NeoForge 21.1.252, Minecraft 1.21.1): 96 mods, 105 files referenced on
  Modrinth, Create Propulsion on CurseForge (`mode =
  "metadata:curseforge"`; it downloads with no manual step), plus its
  config, KubeJS, menu assets and resource pack.
- **Hosted** by a Cloudflare Worker with static assets, built by Workers
  Builds from the private repository; it serves only the chapter folders
  (`kapital-packs#5` and `#6`). `pack.toml` and `index.toml` carry
  `Cache-Control: no-cache`.
- **`tools/import_instance.py`** regenerates a pack from a Prism instance. It
  matches jars and zips on Modrinth by hash, then CurseForge through `packwiz
  curseforge detect`, skips disabled mods, caches and per-player state, and
  marks server-only mods `both` because singleplayer needs them.
- **Files are stored byte for byte** (`.gitattributes`: `* -text`). The first
  commit normalised line endings and broke every `index.toml` hash; do not
  reintroduce `text=auto`.
- **No file over 25 MiB.** The Frangfurd title video (kapital-packs#1) is a
  two-pass 4K60 re-encode at a 40 Mbit/s target, which x264 settled at
  31 Mbit/s, 11.7 MB. The master is
  `D:\Private\Projects\Videogames\Minecraft\Projects\KapitelKapital\assets\intro\frangfurd\Frangfurd-Intro.mp4`.
- **Frangfurd's fullscreen start** (#46): with NeoForge's early window on,
  the title screen could stay in the bottom-left corner of a black screen,
  drawn at 854x480, because the game missed the resize to fullscreen.
  Frangfurd 1.0.0 turned the early window off (`earlyWindowControl = false`),
  which #45 cannot hide in time. kapital-packs#3 turns it back on; the
  launcher fixes the glitch with its one-pixel nudge at the handover (#93). A
  start from Prism directly still gets the glitch now and then.
- **This repository is public**, so everything in it is: the manifest with
  the tunnel addresses, the art, the checklists. Nothing secret was in it
  (S1.2). `kapital-packs` and the wiki are still private.
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
- **Its packwiz sync is on** (`OverrideCommands=true`), syncing from
  `http://localhost:8080/pack.toml`, so Play needs `packwiz serve` running in
  `kapital-packs/frangfurd`. With it down the pre-launch sync fails, Prism
  opens its console with the error, and before #106 the card waited ten
  minutes (#103, seen 2026-10-02). To switch the instance to the hosted pack,
  clear Developer, "Local pack for Frangfurd" on the settings screen; the
  instance keeps its own command until it is reinstalled.
- `%APPDATA%\KapitalLauncher\settings.json` carries
  `"packOverrides": {"frangfurd": "http://localhost:8080/pack.toml"}` and
  `"holdGameWindow": true` from the #45 tests. The loading splash is on by
  default on Windows and off on macOS until #30.
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
  request unless its build watch paths are set (`site/README.md`, #29).

## Things a fresh session will trip on

- **aislop does not run on the author's PC** (no ruff 0.16.7 on `PATH`), so
  the local suite skips it and CI is the first to judge. In a cloud container
  a venv with `ruff==0.16.7` on `PATH` makes it run; it scored 100 on every
  branch of 2026-10-02.
- **The `pr-labelled` gate fails once on every pull request opened by the
  API** before its labels are added, then passes on the runs the label events
  trigger. The failed run stays in the list; it is not a problem to fix.
- **`wails generate module` works in a cloud container** (Linux, no WebKitGTK
  headers needed for it) and leaves a clean tree when the bindings are current.
  `wails build -platform linux/amd64` does not: it stops at the missing
  `gtk+-3.0` and `webkit2gtk-4.0` headers, and it drops the executable bit on
  `frontend/wailsjs/runtime/` files, which `git checkout` restores.
- **Settings written behind the store's back are lost**: the frontend's
  settings store saves its own copy (on a chapter switch, for one), so a field
  set through `SaveSettings` from the console disappears. Change settings
  through the settings screen.
- **Prism's exit code says nothing about a launch**: with
  `ShowConsoleOnError` on, a failed launch keeps Prism open on its console,
  and closing that window exits 0. A `--launch` handed to a Prism already
  running exits 0 at once, writing no log; the running one logs the failure
  in the same root's `logs/PrismLauncher-0.log`, which #106 follows. On
  Windows with the splash on, the launcher now hides that console and shows its
  own failure view with a "Show Prism's console" button (ADR-0012 amendment);
  the Prism stays alive behind it until the next Play, Stop or quit. The title
  prefix `Console window for`, the show and foreground and Prism exiting on
  `WM_CLOSE` are unverified on real hardware, and a player's own Prism with
  `ShowConsole=true` would read as a failed start (the managed root seeds it
  off).
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
- **`.claude/suite-check.py` needs `PYTHONUTF8=1`** on the PC once any
  test prints a non-cp1252 character (a `✓`, an em dash in a diff): the
  vendored runner decodes subprocess output as cp1252 and dies with
  `UnicodeDecodeError` instead of reporting the failing check.
- **`wails dev`'s Go watcher did not rebuild** after edits to `backend/`
  twice on 2026-09-30; the frontend reloaded, the bindings did not. Restart
  it. A `wails dev` from an earlier session may still hold port 34115; the
  `wails-dev` preset in `.claude/launch.json` starts a fresh one with live
  bindings.
- **jsdom has no `AnimationEvent`**, so React listens for
  `webkitAnimationEnd` there: a test that must end a CSS animation
  dispatches that name (`ChapterStage.test.tsx`, `App.test.tsx`'s
  `switchTo`), or the leaving card stays mounted.
- **Vite keeps a stale `wailsjs/go/main/App.js`** after `wails generate
  module` adds a method; restart the frontend dev server or the page fails
  with "does not provide an export".
- **Git Bash heredocs collapse backslashes** in Python and TypeScript written
  through them. Write files with the editor tool, not `cat <<'EOF'`, when a
  backslash or an apostrophe inside quotes matters.
- **Git Bash's `sed -i` strips CRLF.** Pack files are stored byte for byte
  and most of Frangfurd's configs have Windows line endings, so a one-line
  `sed` rewrites the whole file. Edit pack files in binary (Python, `rb`/`wb`)
  and check `git diff --stat` shows one line.
- **`gh pr edit --body-file` with an empty file wipes the description.**
  Write the body with the editor tool instead.
- **"The process cannot access the file" when renaming an instance folder:**
  look for another Claude Code session whose working directory is inside it
  (`list_sessions` shows each `cwd`), before blaming Explorer or Prism.
- **`wails dev` serves the app with live Go bindings at
  `http://localhost:34115`**, so the browser pane can drive the real backend
  (Install, Play) without the Wails window. Wails' own `ipc.js` logs one
  harmless `reading 'nodes'` error there.
- **`packwiz serve --refresh=false` reads `index.toml` once at start**:
  restart it after every `packwiz refresh`, or it serves the old index.
- **Symlinks:** the PC cannot create them without a privilege the Windows CI
  runner has, so a test can pass there and fail in CI; the symlink tests in
  `managedprism_test.go` skip on Windows for that reason. Check CI, not only
  the local run.
- **Prism facts** used by the managed Prism (the setup-wizard conditions, the
  updater's `prismlauncher_update.cfg`, `m_rootPath` being the program
  folder on Windows, the console keys, the launch step order, `Task::emitFailed`)
  were read from Prism 11.1.1's source and are cited in ADR-11, ADR-2's fifth
  amendment and #106. Re-read them when Prism's major version changes. On
  macOS Prism updates through Sparkle, so the `prismlauncher_update.cfg` seed
  has no effect there; whether Sparkle starts on the `--launch` path is
  `[verify]` on the iMac.
- **`[verify]` on a real Mac:** #30. The author's 2017 Intel iMac covers it;
  the bundle layout, its symlinks and the executable path were checked
  against Prism 11.1.1's macOS zip, and #107's universal build is what to
  install there.
- **The Wails CLI rewrites `go.mod` to its own version.** Install v2.16.0,
  and check `git status` after the first `wails dev`
  (`.claude/rules/builds-and-releases.md`). `build.yml` installs the version
  `go.mod` names and fails if the build changed `go.mod` or `go.sum`.
- **pnpm's release-age policy** refuses packages younger than a day, in both
  `pnpm-workspace.yaml` files, on purpose (`.claude/rules/dependencies.md`).
- **In a cloud container** every check runs, aislop included, with ruff
  0.16.7 from a venv on `PATH`. Java there follows a POSIX locale, so run
  packwiz-installer with `LC_ALL=C.UTF-8` (#32). The bootstrap's update check
  against `api.github.com` gets a 403 there; fetch `packwiz-installer.jar`
  from its release page and pass `--bootstrap-no-update`. `raw.githubusercontent.com`
  answers for tags and branches of public repositories (`11.1.1` and
  `develop` of Prism did; the bare `11.1` ref does not exist), `wails.io` and
  `api.github.com` refuse, and the Wails docs are readable as MDX under
  `wailsapp/wails/website/docs/` at the tag.
- **The repository is public since 2026-09-30 evening**, made so because the
  private allowance of Actions minutes ran out mid-stack and every job failed
  unstarted. Actions on standard runners is free now, and CodeQL and
  Scorecard run (#88); `build.yml` adds a Windows and a macOS job to every
  pull request.
- The vendored files (`.claude/suite-*.py`, three workflows, the notes
  generator, `.aislop/base.yml`) are copied from `kollektiv-mc/Kollektiv` by
  hand, and `codeql.yml` and `scorecard.yml` from `kollektiv-mc/Konnekt`.
  Re-copy to update.
