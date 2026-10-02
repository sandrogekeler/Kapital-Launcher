---
paths:
  - ".github/workflows/**"
  - "version.go"
  - "wails.json"
  - "build/**"
---

# Builds and releases

`version.go`'s `Version` is the single source of the app version, mirrored in
`wails.json`'s `info.productVersion`. The `-dev` suffix marks a build that is
not a release. A release stamps it with `-ldflags "-X main.Version=<tag>"`.

`.github/workflows/build.yml` packages both targets with `wails build`. On every
push to `main` and every pull request it uploads `windows-amd64` (the bare
`.exe`) and `macos-universal` (the `.app`, zipped with `ditto`, which keeps the
modes `upload-artifact` would flatten) as workflow artefacts for 14 days, named
`kapital-launcher-<short sha>-<target>`. A release is cut from the Actions tab:
run it with a `version` input, `vX.Y.Z[-alpha.N|-beta.N]`. The tag is
validated, the build stamps `main.Version` with it and `wails.json`'s
`productVersion` with its numeric part, and the `release` job attests the zip
and exe with `actions/attest`, writes `checksums.txt` and publishes a GitHub
Release at that commit, a prerelease when the tag has a suffix. No
code-signing certificate (ADR-6). The body is `.github/release-body.md` (the
Minecraft disclaimer, the SmartScreen and Gatekeeper steps, how to verify)
followed by `release-notes.py`'s notes, or GitHub's generated ones when it has
no baseline. The Wails CLI is installed at `go.mod`'s version and the build
fails if it changed `go.mod`. The macOS build sets `CGO_CFLAGS` and
`CGO_LDFLAGS` to `-mmacosx-version-min=12.0` (Wails hardcodes 10.13) to match
`LSMinimumSystemVersion` in `build/darwin/*.plist`: Go 1.26 and Prism 11 both
need macOS 12.

## The toolchain directive is what CI runs

`go.mod`'s `toolchain go1.26.x` is the exact Go that `setup-go` installs in
CI, so it is also the standard library `govulncheck` scans. When the scanner
reports a fix in a newer patch release, bump the directive; the `go 1.26.0`
line is the language minimum and stays. A machine on an older patch
downloads the pinned one on first use.

## The Wails CLI version is the module version

`wails dev` and `wails build` rewrite `go.mod` to the CLI's own Wails version
and run `go mod tidy` before doing anything else. A CLI older than
`go.mod`'s `github.com/wailsapp/wails/v2` therefore downgrades the module in
the working tree, silently. Install the CLI at the version `go.mod` names
(`go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0`) and check
`git status` after the first `wails dev` on a machine.

## CI

`.github/workflows/ci.yml` runs the frontend gates on Ubuntu, the Go gates on
Windows and on an Apple Silicon macOS runner (the two targets, ADR-7) with the
frontend built first because `main.go` embeds `frontend/dist`, the release
notes generator's tests, and the vendored runner's `invariants`, `generated`
and `memory` sections with `--require-runnable`.

Beside it: `pr-labelled.yml` (one `type:` and one `area:` label),
`pr-copy.yml` (title in sentence case, no em dash in title or body),
`aislop.yml` (the aislop gate at 100, with ruff pinned) and
`issue-priority.yml` (the form's answer becomes a `p*` label). The first,
third and fourth are vendored from Kollektiv and never edited here. `codeql.yml`
and `scorecard.yml` are vendored the same way from Konnekt.

Every action is pinned to a commit SHA with the version as a trailing comment,
and every workflow declares `permissions: contents: read`. Dependabot keeps the
pins current.

## `build/`

`appicon.png` is the Wails template's placeholder icon until Kapital has one;
`build/windows/` and `build/darwin/` are the template's manifest and plist,
which `wails build` reads and regenerates if deleted. The Windows `info.json`
reads `wails.json`'s `info` block, which is where the company name, product
name and the disclaimer comment come from.

Linux builds need `-tags webkit2_41` on distributions on the 4.1 side of
WebKitGTK; `wails doctor` says which.
