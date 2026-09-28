# ADR-0007: Target operating systems

**Status:** accepted, 2026-09-28.

## Context

The choice decides Prism path detection, the CI build matrix, and
packaging. The author answered: **Windows 11 and macOS on Apple Silicon.**
No Intel Macs, no Linux.

## Decision

- **Two targets: Windows 11 (amd64) and macOS (arm64).** CI's Go gates run
  on `windows-latest` and `macos-latest`, the latter an Apple Silicon
  runner, so both binaries compile on every push. The first release ships
  a Windows executable and a macOS app bundle.
- **Detection stays written for Linux too** (`backend/services/prism.go`,
  including Flatpak). It costs a few lines and a test, nothing in the code
  prevents a Linux build, and removing it buys nothing. It is simply not
  promised, not built in CI, and not released.
- macOS-specific work still ahead: the `.app` bundle's plist in
  `build/darwin/`, and whether Gatekeeper's prompt on an unsigned bundle is
  acceptable (ADR-6 says yes for now).

## Consequences

- The issue forms offer Windows 11 and macOS (Apple Silicon) and a
  "something else".
- A Linux report is answered as best effort.
