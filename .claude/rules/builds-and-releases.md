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

## CI

`.github/workflows/ci.yml` runs three jobs: the frontend gates on Ubuntu, the Go
gates on Windows (the primary target, ADR-7) with the frontend built first
because `main.go` embeds `frontend/dist`, and the vendored runner's
`invariants` and `generated` sections with `--require-runnable`.

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
