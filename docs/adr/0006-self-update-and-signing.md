# ADR-0006: Self-update and code signing

**Status:** proposed. Roadmap, milestone 7.

## Context

Unsigned Windows builds trigger SmartScreen; unsigned macOS builds hit
Gatekeeper. A code-signing certificate costs money every year, and this is a
personal tool for a handful of people who know where it came from.

## Proposal

- **Manual updates first.** A GitHub Release per version, artefacts attested
  with `actions/attest` and a `checksums.txt`, the way Konnekt's release
  workflow does, so provenance is verifiable without a certificate.
- **No certificate** for now. The SmartScreen and Gatekeeper prompts are
  acceptable for personal use, and the release notes say how to get past
  them and how to verify the attestation.
- An in-app update check (GitHub Releases API, compare against `Version`) is
  a later step, and an in-place updater later still, if ever; Konnekt's
  `selfupdate` path is the reference when it comes.

## Amendment, 2026-10-04: installers, and signing reopened

The first beta goes to players through the download site, which is the
"Reopen when" below. Decided with the author:

- **Installers** (issue 177): an NSIS setup on Windows, for the one user, and
  a disk image on macOS. Both are attested like the files before them.
- **Windows signing through SignPath Foundation** (issue 179), free for
  open-source projects. Its terms want an OSI licence on the repository and a
  release already published in the form to be signed, so the first beta ships
  unsigned and the application follows it. The publisher will read SignPath
  Foundation. No certificate clears SmartScreen on day one; reputation builds
  with downloads.
- **No Developer ID for the beta.** Gatekeeper is only satisfied by Developer
  ID and notarization, which is the Apple Developer Program at 99 USD a year.
  Players use Privacy & Security, Open Anyway, as the release notes say.

## Reopen when

Friends beyond the author install it, or a build is distributed anywhere other
than the repository's own releases.
