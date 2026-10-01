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

There is no release workflow yet (Roadmap, milestone 7). When one arrives it
follows Konnekt's: cut from the Actions tab, tag `vX.Y.Z[-alpha.N|-beta.N]`,
artefacts attested with `actions/attest` and a `checksums.txt`, no code-signing
certificate (ADR-6), and the release body carries the Minecraft disclaimer.

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
