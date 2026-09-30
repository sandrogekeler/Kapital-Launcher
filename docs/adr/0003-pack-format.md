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
- Hosting is a static host that serves `pack.toml`, `index.toml`, the mods'
  metadata and every file the pack carries itself (configs, scripts, its own
  resource pack); GitHub Releases or Cloudflare Pages. For Frangfurd that is
  426 files besides the metadata, about 107 MB, all of it public once hosted
  (#35). The manifest schema already has `pack.packwiz` and `pack.mrpack`
  slots.

## Authoring: a repository, not an admin panel

The question came up whether packs should be managed from an admin panel
that adds and removes mods and tests locally, or fetched by the launcher
from a hosted `.mrpack` and manifest. The second, and the reasons are what
packwiz already is:

- **The pack source is a git repository** of packwiz TOML, one folder per
  chapter. Adding a mod is `packwiz modrinth add <slug>` (or `curseforge
  add`), removing is `packwiz remove`, and `packwiz refresh` rewrites
  `index.toml` with hashes. Every change is a commit with a diff, which is
  the audit trail an admin panel would have to build.
- **Local testing is `packwiz serve`**, packwiz's built-in HTTP server for
  exactly this: point an instance's pre-launch installer at
  `http://localhost:8080/pack.toml` and launch. The launcher gains a
  developer setting for that override (Roadmap, milestone 4), so a pack can
  be tried from a working copy before it is pushed.
- **Publishing is a push.** A static host serves `pack.toml` and
  `index.toml`; `packwiz modrinth export` produces the `.mrpack` a fresh
  install imports. The launcher's manifest carries the two URLs and nothing
  else changes.

An admin panel would be a second application with its own hosting, auth and
database, to do what the CLI and a git remote do for one person and a few
friends. It is the wrong size. If the pack authors ever stop being people
who use git, the panel is a front-end over the same repository, not a
replacement for it.

## Open

- How Prism's import handles an instance whose name already exists. Answered
  from Prism 11.1.1's source on 2026-09-30 (the comment on #22 cites the
  lines): `-I` always opens the New Instance dialog and waits for OK, the
  folder name comes from that dialog's name field, and a collision gets a
  `(1)` suffix, never an overwrite. So an import cannot promise the
  `kapital-<id>` folder `--launch` needs; #22 decides what the launcher does
  instead.
- Whether `packwiz-installer-bootstrap.jar` is fetched by the launcher (then
  hashed) or by the pack import.
