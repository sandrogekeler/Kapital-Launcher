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

## Reopen when

Friends beyond the author install it, or a build is distributed anywhere other
than the repository's own releases.
