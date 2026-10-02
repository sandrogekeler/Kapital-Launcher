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
its process. The only other processes are macOS's `codesign`, with fixed
arguments, in `verify_darwin.go`, and macOS's `open`, with the absolute path of
a folder that exists as its only argument, in `openfolder_darwin.go`. Windows
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
which ends the run as `failed` with a `Reason` (ADR-2, fifth amendment). It
starts no process. Stop (`gametracker_stop.go`) is a request to a run's own
goroutine that ends only the two processes the run found itself, the launcher's
Prism and the game's Java, by pid and never by name: the Java is ended, else
Prism is asked to close (`WM_CLOSE` to its own windows on Windows, `SIGTERM` on
macOS) and ended if it is still there five run-steps later, and the run ends
`crashed` or `failed` with the reason `stopped` (S3.9). On macOS it holds one OS activity per run
(`activity_darwin.go`: `NSProcessInfo`'s `beginActivity`, from `begin` to the
end of `loop`) so App Nap does not coalesce its waits while the launcher is
minimised; it reads nothing.

The window holder (`gamewindow*.go`, #45, Windows only; the loading splash,
#43, is what turns it on for a run) hooks the show events of the game's own
process and hides and shows its `GLFW30` window through user32. The handover is
the resource reload beginning: the window is shown and given the foreground,
then, once it reports its real rectangle, a fullscreen one is made one pixel
shorter and back (queued to the game's thread, #46).
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

The loading card (#43, #97) is a window of its own: `backend/splashhost` is one
borderless window with a webview of its own per OS (`host_windows.go`,
`host_darwin.go`, `host_other.go`, which has none), behind `Host` (`Open`,
`Update`, `Close`). It shows `frontend/splash.html` from the embedded build,
which the host serves through its in-memory resource handler at
`http://splash.localhost/` (Windows) or `kapital-splash://app/` (macOS), from
`Page.Assets` (`AssetsFrom`, which refuses `..`, anything that is not a file and
any type it does not list); there is no local server and no network. Go pushes
the card's state (`splashhost.State`) with `window.kapitalSplash.update`, and the
page posts back one of three actions, `leave`, `openFolder` and `copyLog`, as a
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
exist, `instance.cfg` last, and never touched again (ADR-2, amendment). In the
managed root it owns it also seeds `prismlauncher.cfg` and
`prismlauncher_update.cfg` once, before Prism first starts. The pre-launch
command it writes runs packwiz-installer headless (`-g`, #95); before a launch
`RewritePreLaunchCommand` (`prelaunch.go`) brings an instance made earlier up to
that command, only when the key is exactly the launcher's earlier template, with
the same URL, and leaves anything else alone (ADR-2, fourth amendment).

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
