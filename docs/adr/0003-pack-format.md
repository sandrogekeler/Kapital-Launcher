# ADR-0003: Pack format and distribution

**Status:** accepted in principle, 2026-09-28; nothing built yet (Roadmap,
milestone 4).

## Context

The handoff (§3, ADR-3) proposed packwiz as the source of truth, with
`packwiz-installer` in Prism's pre-launch command for day-to-day sync (option B)
and a `.mrpack` import for a fresh install (option A). Verified 2026-09-28:
packwiz is MIT, exports Modrinth and CurseForge packs, and its installer is
built for auto-updating MultiMC-family instances. Prism's custom commands
substitute `$INST_JAVA`, `$INST_DIR`, `$INST_MC_DIR`, `$INST_ID` and
`$INST_NAME`, and warn that `$INST_JAVA_ARGS` breaks on arguments with spaces.

## Decision

As proposed, with two things stated more precisely than the handoff did:

- **The pre-launch command is a template in Go**, of the form
  `"$INST_JAVA" -jar "$INST_MC_DIR/packwiz-installer-bootstrap.jar" <pack.toml url>`.
  The manifest contributes only the URL, already validated as https on an
  allowlisted host. No manifest field can ever carry a command.
- **Hashes are required.** `.mrpack` files carry per-file hashes and packwiz's
  `index.toml` does; a file whose hash is missing or does not match is
  refused, not skipped.
- Mods are referenced by their Modrinth or CurseForge download URL and never
  re-hosted. Prefer Modrinth where a mod is on both, and expect some
  CurseForge mods to opt out of third-party distribution.
- Hosting is a static host that serves only manifests and `index.toml`;
  GitHub Releases or Cloudflare Pages. The manifest schema already has
  `pack.packwiz` and `pack.mrpack` slots.

## Open

- How Prism's import handles an instance whose name already exists. Observe it
  before deciding whether a fresh install can be non-interactive at all.
- Whether `packwiz-installer-bootstrap.jar` is fetched by the launcher (then
  hashed) or by the pack import.
