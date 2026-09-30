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
function that builds the argument array, and `Launch` starts the game. The only
other process is macOS's `codesign`, with fixed arguments, in
`verify_darwin.go`. There is no shell anywhere, and the `shell never sees a
command string` invariant holds it.

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
`prismlauncher_update.cfg` once, before Prism first starts.

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
