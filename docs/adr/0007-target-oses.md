# ADR-0007: Target operating systems

**Status:** proposed; needs the author's answer to "which OSes are needed?"

## Context

The choice decides Prism path detection, the CI build matrix, and packaging.
Konnekt's release builds Windows first and Linux on the snapshot channel; the
author's own machine is Windows.

## Proposal

- **Windows is the primary target**: CI's Go job runs on `windows-latest`, and
  the first release is a Windows executable.
- **Detection is written for all three** (`backend/services/prism.go`:
  standard install locations for Windows, macOS and Linux, plus Flatpak on
  Linux), because it costs a few lines and a test each, and because
  `runtime.GOOS` is the only switch. Each path is marked `[verify]` until it
  is observed on a real install.
- macOS and Linux builds are added when someone will run them.

## Open

Which OSes the friends on the servers use. Until answered, nothing beyond
Windows is promised, and nothing in the code prevents the others.
