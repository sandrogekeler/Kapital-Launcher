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

`.github/workflows/build.yml`, named Release, is the one place both targets
are packaged with `wails build`. It runs only from the Actions tab ("Run
workflow"), never on a push or a pull request; those are judged by `ci.yml`.
Its fields: `version` (`X.Y.Z`), `channel` (`beta`, `alpha` or `stable`),
`number` (the N of a prerelease) and `dry_run`. Together they make the tag,
`vX.Y.Z[-alpha.N|-beta.N]`. It uploads `windows-amd64` (the NSIS setup and the
bare `.exe`) and `macos-universal` (a disk image holding the `.app` and a link
to Applications) as workflow artefacts for 14 days, named
`kapital-launcher-<tag>-<target>`; a dry run stops there and publishes
nothing, which is how a change to the workflow or to `build/` is tried from
its branch. The tag is validated and refused when taken, the build stamps `main.Version` with it and `wails.json`'s
`productVersion` with its numeric part, and the `release` job attests the disk
image and both exes with `actions/attest`, writes `checksums.txt` and publishes a GitHub
Release at that commit, a prerelease when the tag has a suffix. No
code-signing certificate (ADR-6). The body is `.github/release-body.md` (the
Minecraft disclaimer, the SmartScreen and Gatekeeper steps, how to verify)
followed by `release-notes.py`'s notes, or GitHub's generated ones when it has
no baseline. The Wails CLI is installed at `go.mod`'s version and the build
fails if it changed `go.mod`. The macOS build sets `CGO_CFLAGS` and
`CGO_LDFLAGS` to `-mmacosx-version-min=12.0` (Wails hardcodes 10.13) to match
`LSMinimumSystemVersion` in `build/darwin/*.plist`: Go 1.26 and Prism 11 both
need macOS 12.

## The installers

Issue 177. Windows: `wails build -nsis -installscope user` runs
`build/windows/installer/project.nsi`, Wails' template with three changes
(the installed file keeps the name `kapital-launcher.exe`, a "Run" tick on the
last page, a note on what the uninstaller leaves). It installs to
`%LOCALAPPDATA%\Programs\Kapital Launcher` with no UAC prompt, runs
Microsoft's WebView2 bootstrapper when the runtime is missing, and its
uninstaller never touches `%APPDATA%\KapitalLauncher` (settings, the managed
Prism, the player's worlds). `wails_tools.nsh` and `tmp/` beside it are
written by every build and ignored. The runner image has no NSIS and Wails
only warns when `makensis` is missing, so the job installs a pinned one and
the copy of the setup is the check that it ran. The installer's version field
takes digits and dots only, so the Windows job always stamps `wails.json`
with the numeric part.

macOS: `hdiutil` makes a compressed image from the bundle and an Applications
link. The bundle identifier is `io.github.sandrogekeler.kapital-launcher`,
written in both plists and checked in the job; it is the app's identity
(preferences, WebKit data) and does not change again.

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
