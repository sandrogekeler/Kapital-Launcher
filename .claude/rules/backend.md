---
paths:
  - "backend/**"
  - "app.go"
  - "main.go"
---

# Go backend conventions

`gofmt` is enforced. Errors are always handled: the `no discarded errors`
invariant in `.claude/suite.json` greps for `x, _ := f()` as well as `_ =`. A
deliberate discard carries a trailing `//nolint:errcheck // reason`.

Diagnostics go through `log/slog`'s package-level functions, never `fmt.Print`
or `println`: a packaged GUI build has no terminal, and `main()` points the
default logger at `kapital-launcher.log` in the app data dir. Nothing that
identifies the user beyond what the log already must carry (paths, the
profile name) is logged, and `services.Redactor` is what a shared copy runs
through.

## Prism

`backend/services/prism.go` runs Prism. The command line is Prism's
documented CLI, quoted at the top of the file; `LaunchArgs` is the pure, tested
function that builds the argument array, and `Launch` starts Prism and returns
its process. `ShowArgs` and `Show` do the same for `--show` (Open in Prism,
issue 190), whose Prism is waited on only to be reaped. The only other processes are macOS's `codesign`, with fixed
arguments, in `verify_darwin.go`, macOS's `open`, with the absolute path of
a folder that exists as its only argument, in `openfolder_darwin.go`, and the
pre-launch sync's Java, the `INST_JAVA` Prism named with the packwiz template's
arguments, in `modsync.go` (`ExecSyncRunner`). Windows
opens a folder with `ShellExecute`, an API call and not a process. Copying the
log (`CopyRedactedLog`) goes through Wails' `ClipboardSetText`, which on macOS
runs `pbcopy` with no argument and the redacted text on its stdin
(`internal/frontend/desktop/darwin/clipboard.go` in Wails): a process of
Wails' and not one this app starts. There is no shell anywhere, and the `shell
never sees a command string` invariant holds it.

The game tracker (`gametracker.go`, `gameproc_*.go`) follows a launched game:
it looks up Prism's child processes (pid, parent pid and name, nothing else)
and waits on the game's Java, and reads the instance's `latest.log` for marker
lines only, never keeping or logging one (ADR-2, third amendment). While the
start waits for the game it also follows Prism's own `logs/PrismLauncher-0.log`
(`gametracker_prism.go`, #103) for one marker, a `LaunchTask` that failed,
which ends the run as `failed` with a `Reason` (ADR-2, fifth amendment). Prism's
own `qtlogging.ini` turns the `launcher.task` category off, Critical included,
so that line is never written by default: the managed root carries a copy of
Prism's rules plus `launcher.task.critical=true` (ADR-11, `seedLogRules`). It
starts no process. Stop (`gametracker_stop.go`) is a request to a run's own
goroutine that ends only the two processes the run found itself, the launcher's
Prism and the game's Java, by pid and never by name: the Java is ended, else
Prism is asked to close (`WM_CLOSE` to its own windows on Windows, `SIGTERM` on
macOS) and ended if it is still there five run-steps later, and the run ends
`crashed` or `failed` with the reason `stopped` (S3.9). A Prism that outlives a
stopped game is closed the same way as the run ends (`closePrismAfterStop`,
#133); a game that crashed by itself keeps its Prism for Show console. On macOS it holds one OS activity per run
(`activity_darwin.go`: `NSProcessInfo`'s `beginActivity`, from `begin` to the
end of `loop`) so App Nap does not coalesce its waits while the launcher is
minimised; it reads nothing.

The one time the launcher reads a line of the game's log is `GameTracker.Report`
(`gametracker_report.go`, `GetRunReport`, ADR-2 sixth amendment): on the
player's request, or when the card is up and a run ends `crashed` or `failed`,
it reads the last 16 KiB of that run's `latest.log` and the name of the newest
crash report written since Play, runs the tail through the redactor with the
player's in-game name learned from the log added, and returns it. It is built
for the call and kept nowhere, and no line of it goes to `slog`; a run with no
game log of its own has none to show, and without a redactor there is no report.

The logs page (`runlogs.go`, `app_logs.go`, `GetRunLogs` and `ReadRunLog`, #155,
ADR-2 eighth amendment) is the other reader of the game folder, on request and
read only: it lists `logs/latest.log`, `logs/*.log.gz` and `crash-reports/*.txt`
by file times, marks the logs of a run a crash report falls in, and reads one
file a 256 KiB chunk at a time, unpacking a `.log.gz` under a cap, through an
`os.Root` on the game folder and only by a name `validRunLogName` accepts. The
chunk goes through the redactor with the in-game name added before it leaves,
and nothing of it is kept or logged.

The live log (`livelog.go`, `WatchLiveLog` and `StopLiveLog`, issue 155) is
that reader kept going for `logs/latest.log` alone: while the page shows it, Go
looks at the file every 500 ms (the one place that polls, as the game tracker
does; the frontend never does), reads what was appended through the same
`os.Root` and name rules, keeps a half-written last line until its newline,
redacts the lines and emits `log:live` (`EventLiveLog`) in events of at most
64 KiB. A shrunk or replaced file, told by its first 128 bytes, emits a reset.
One follower at a time, under the app's run context.

The window holder (`gamewindow*.go`, #45, Windows only; the loading splash,
#43, is what turns it on for a run) hooks the show events of the game's own
process and hides and shows its `GLFW30` window through user32. Every hide,
show and resize is queued to the game's thread (`ShowWindowAsync`,
`SWP_ASYNCWINDOWPOS`) and never waited on: Forge 1.19.2 reads no message while
it loads mods, and a hide that waited on it outlived the handover (#132). The
handover is the resource reload beginning: a hide still queued is let land, the
window is shown and, once it reports its real rectangle, given the foreground
and, if fullscreen, made one pixel shorter and back (#46).
It reads the class, owner and rectangle of the game's window, never its title,
and starts no process.

The same holder, over the same hook and callback, also hides Prism's "Please
wait" progress dialogs (`gamewindow_dialogs_windows.go`, #95) for the Prism the
launcher started, while the splash is on: from the start of the run, when
`TrackRequest.HoldWindow` is set, to the game's handover, where the hook ends
and nothing is shown (Prism closes them itself). A run that ends first, or
whose Prism exits first, shows back any that still exist. These are told apart
by a title that begins `Please wait`, the one title the launcher reads (ADR-0012's
no-title rule is about the game's window); a sign-in, an error or a translated
Prism's dialog matches nothing and stays in view. Each release logs one
`prism dialogs` line with the chapter, the hides and whether any were shown back,
never a title.

Prism's console window is held the same way (`gamewindow_console_windows.go`,
ADR-0012 amendment), started with the dialogs' hold and on the same condition,
and matched by a title that begins `Console window for`. It is not released at
the handover or when the run ends: the console appears at about the moment a
failed start ends, and its output exists nowhere else. The hold lives on a
per-chapter record in the tracker (`gametracker_console.go`), where it also is
the second failure signal (a console seen while the start waits, with no game
log of its own, ends the run `failed` with the reason `launch`, after a second
for Prism's own log to give a better one), and `consoleAvailable` in the run
report. `ShowPrismConsole` and the card's `showConsole` action end the hook and
show the windows. A Prism left alive on a console is the launcher's to close,
with Stop's routine plus `WM_CLOSE` to the hold's own (hidden) windows: when the
next Play of the chapter begins, on Stop and when the launcher quits
(`GameTracker.Shutdown`, bounded). A Prism with no console left is not touched.
Windows only; the title is read through `GetWindowTextW`, never kept or logged.

The loading card (#43, #97) is a window of its own: `backend/splashhost` is one
borderless window with a webview of its own per OS (`host_windows.go`,
`host_darwin.go`, `host_other.go`, which has none), behind `Host` (`Open`,
`Update`, `Close`). It shows `frontend/splash.html` from the embedded build,
which the host serves through its in-memory resource handler at
`http://splash.localhost/` (Windows) or `kapital-splash://app/` (macOS), from
`Page.Assets` (`AssetsFrom`, which refuses `..`, anything that is not a file and
any type it does not list); there is no local server and no network. Go pushes
the card's state (`splashhost.State`) with `window.kapitalSplash.update`, and the
page posts back one of four actions, `leave`, `openFolder`, `copyLog` and `showConsole`, as a
string of JSON: `ParseMessage` drops anything else, and a message carries no
argument. `protocol.go` is the whole contract.

`SplashCard` (`services/splashcard.go`) drives it on the tracker's phase changes
and keeps the launcher's window out of the way: `Begin` before Prism runs
(opens the card centred on the launcher's window, then minimises the launcher;
when the card cannot open it says so and the run holds nothing), `Observe` per
game event, `Handover` from `TrackRequest.OnHandover` on Windows once the
holder's foreground release is done, and `Leave`. On macOS there is no window
hold, so the card closes at the game's `window` phase instead. The launcher
comes back when the game ends, showing how it ended; a crash or failure before
the handover keeps the card for the error until the player leaves it. The
launcher's own window is only asked where it is, to minimise and to come back
(`launcherWindow` in `app_splash.go`).

`managedprism.go` gets Prism for a player who has none, on approval only
(ADR-11): downloads are verified by digest and signature before anything is
placed, and a managed Prism always runs with its own `--dir` root.

Detection is injected (`lookPath`, `getenv`, `stat`, `run`) so it is tested on
a machine with no Prism. A path that has not been observed on a real install is
marked `[verify]` in a comment; clearing those is Roadmap milestone 2.

The launcher writes one thing into a player's Prism data directory: a
chapter's instance folder, created by `InstanceCreator` only when it does not
exist, `instance.cfg` last, and touched again only in the keys the amendments
name (ADR-2, amendment). In the managed root it owns it also seeds `prismlauncher.cfg`,
`prismlauncher_update.cfg` and `qtlogging.ini` once, before Prism first starts
(the last also before a launch, when absent, for an earlier install). The pre-launch
command it writes is the launcher's own copy in a sync mode,
`"<data dir>/sync/kapital-launcher" --prelaunch-sync <pack URL>` (issue 156, ADR-2
ninth amendment; `prelaunchcommand.go` has the three templates the launcher has
written and reads them all, the URL always last). `main()` branches on the flag
before it opens a log or a window and `services.RunSync` (`modsync.go`) does the
work: it puts the player's disabled mods back from `.jar.disabled`, runs
packwiz-installer headless (`-g`, #95) as an argument array with the arguments of
the packwiz template (`packwizSyncArgs`), and puts them away again, with a journal
(`kapital-disabled.json`) written first because Prism's cancel is a hard kill. It
reads `INST_MC_DIR`, `INST_JAVA` and `INST_ID` from Prism's environment, prints
ASCII `kapital-sync:` lines, touches only regular files directly in the instance's
`mods` folder through an `os.Root` by the `ModJarName` shape (`mods.go`), and is
the one place that starts Java. The copy (`synccopy.go`, `SyncCopy`) is refreshed
when the version differs, atomically, in `sync/` or, for a dev build, `sync-dev/`;
with none the instance keeps the packwiz command. Before a launch
`RewritePreLaunchCommand` (`prelaunch.go`) brings an instance made earlier up to
the current command, only when the key is exactly one of the launcher's templates
with the same URL, and leaves anything else alone (ADR-2, fourth and ninth
amendments). On the player's request `SwitchPackSource` (`packswitch.go`,
`SetPackSource`) rewrites the same key between the manifest's pack and the
loopback override, under the same template check (ADR-2, seventh amendment).
`SetModsDisabled` and `GetChapterMods` (`app_mods.go`) are the settings page's
side: the list lives in settings (`disabledMods`), is validated against the mods
folder, applied at once, and refused while the game runs.

The map check (`maps.go`, `app_map.go`, `CheckChapterMap`, issue 161, ADR-4's
third amendment) is the one network call for a chapter's web map: a GET of the
address the manifest names for the chapter (a `*.tun.ply.gg` tunnel with a port,
the one http URL a manifest may carry), 5 s, no redirect followed, at most 4 KiB
of the body read and dropped. Any HTTP status is reachable. It returns a
`models.MapStatus` and never an error for a map that does not answer; the page
frames the map only when it did (SECURITY_CHECKLIST S5.4). `CheckMap` holds the
address to the manifest's rule again, so a caller cannot widen it.

The wiki art (`wikiart.go`, `wikidraw.go`, `GetWikiShots` and `GetWikiArtStats`,
#141 and issue 172) is the one place that downloads pictures. The wiki's lore
export lists screenshots (`screenshots[]`) and each page's pictures
(`pages[].images`, `/vault/images/<file>`; an older wiki build has none, which
leaves the screenshots alone). A chapter's pool is its era's screenshots plus its
era's pages' pictures; `drawPictures` takes up to the player's number of them
(`AppSettings.WikiPictures`: 5, 10 or 20, 10 when unset, 0 for all), in rounds
over the pages (a screenshot is grouped with the page it shows, one with none
alone): each round a random order of the pages that have pictures left and a
random one of each, so no page gives a second before every page has given one.
The random source is seeded by the local date and the chapter id (`seedKey`): the
set holds through a day and is another on the next, and `Shots` draws again when
its number or the date differs from the last call (the settings page calls
`GetWikiShots` after a save). Only the drawn are downloaded, through `fetchArt`'s
checks (the manifest's wiki host, WebP, PNG or JPEG by sniffing, 4 MiB, no
redirect), into `wiki-art/<world>/<file>` for a screenshot and `wiki-art/pages/<file>`
for a page picture (`pages` is no world a screenshot may name, so names cannot
collide). A cached picture drawn again is kept and asked about once per start, and
`pruneArt` removes every other file when the wiki was reached; from the cached
export (offline) the pool is what is already cached and nothing is pruned. The
route `/wiki-art/` (`ArtMiddleware`, `artRoute`) answers GET and HEAD for the two
name shapes only, reading through an `os.Root`. `GetWikiArtStats` gives the
cache's mean picture size (100 KB while it is empty) and each era's pool size, for
the settings screen's estimate.

## Data shapes

Live in `backend/models/`, JSON tags in `camelCase`, and Wails generates the
TypeScript. A pointer field is optional on the TS side; `frontend/src/types/`
mirrors that by hand and `pnpm typecheck` fails where the two disagree. Keep
`design/launcher.schema.json`, `models.Manifest` and `ValidateManifest` in step:
a field added to one is added to all three in the same change.

## Files

Settings are written with `writeFileAtomic` (temp file, rename) at `0600`.
`services.DataDir()` is the only place the app data dir is computed.

## Tests

Every service ships with tests, table-driven where it fits. The root package
tests (`app_test.go`) run under `go test ./...` and need `frontend/dist/` to
exist, which `.gitkeep` guarantees on a fresh clone.

A test that opens a real window (the window holder's child windows, the
loading card's WebView2 window) starts with `testwindows.Require`: it runs in
CI, and locally only with `KAPITAL_WINDOW_TESTS=1`, because those windows take
the foreground while they run (#138). Run them locally after touching
`gamewindow*.go` or `backend/splashhost`, with the desktop left alone for the
half minute they take.
