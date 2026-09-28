# Kapital Launcher: Roadmap

Direction and sequencing. Work items are GitHub Issues; this file says what
comes in which order and what is out of scope, so an issue can be judged
against it. The milestones are docs/HANDOFF.md §7, adjusted where building
the scaffold changed the picture.

## Milestone 1: Scaffold (done, 2026-09-28)

- [x] Wails v2.16.0 + React 19 + TypeScript + Vite + Tailwind v4, mirroring Konnekt
- [x] Design tokens as data (`design/tokens.json`) generated into CSS, TS and Go
- [x] The reference layout: sidebar with chapter nav and engine card, hero,
      action bar, three panels, disclaimer
- [x] Chapter manifest with schema, Go validation, embedded default
- [x] Prism detection (settings, PATH, standard locations, Flatpak) and launch
      through an argument array, with tests
- [x] Settings persisted as JSON in the app data dir, owner-only
- [x] Log file with rotation, and a redactor for sharing it
- [x] agent_docs, path-scoped rules, `.claude/suite.json`, vendored check runner, CI

## Milestone 2: Detect Prism, for real

- [ ] Confirm the standard install paths on Windows, macOS and Linux against real
      installs; the code marks each `[verify]`
- [ ] Confirm `prismlauncher --version` prints to stdout on a Windows GUI build
- [ ] Settings screen: Prism executable, Prism root, profile name, theme
- [ ] Read the instances folder under the chosen root to know whether a chapter's
      instance exists, so Play can say "Install" instead

## Milestone 3: Launch end to end

- [ ] `prismlauncher [-d root] -l <id> [-s addr] [-a profile]` against a real
      instance, all three chapters
- [ ] Behaviour when Prism is already running (single-instance handling is
      Prism's; observe what `--launch` does then)
- [ ] Open instance folder action

## Milestone 4: Packs

- [ ] Author the Frangfurd pack in packwiz; host `pack.toml` and `index.toml`
- [ ] Fresh install: `prismlauncher -I <mrpack url>`; observe the import dialog
      and what happens to an instance with the same name
- [ ] Day-to-day sync: pre-launch command built locally from a template,
      `"$INST_JAVA" -jar packwiz-installer-bootstrap.jar <pack.toml>`, never
      from the manifest
- [ ] Verify every download by hash; refuse a mismatch
- [ ] Update pack button and sync state line

## Milestone 5: Manifest from the site

- [ ] `launcher.json` endpoint on the Kapitel Kapital Astro site, in the shape of
      `design/launcher.schema.json`
- [ ] Fetch with a unique User-Agent, cache, fall back to the bundled copy offline
- [ ] Screenshots by URL; CSP `img-src` grows by the site's host

## Milestone 6: Lichdenstein server status

- [ ] Server List Ping to the manifest's address only; player count and MOTD
- [ ] Decide offline behaviour: disable Join, or launch the visuals pack anyway
      (docs/adr/0005-server-status.md)

## Milestone 7: Polish

- [ ] Changelog panel fed by the manifest
- [ ] Copy redacted log action (`services.Redactor`)
- [ ] About screen: version, disclaimer, licences (fonts, lucide, Prism's GPL notice)
- [ ] Forward frontend render errors to the Go log
- [ ] Release workflow with attested artefacts; code signing decision (ADR-6)

## Out of scope, and why

- **Handling Microsoft sign-in.** Needs an approved Azure app id that solo
  developers are refused. Prism has one. (HANDOFF §1)
- **Bundling or modifying Prism.** GPL-3.0 duties, and a fork cannot reuse
  Prism's client id. The app points at prismlauncher.org instead. (HANDOFF §5)
- **Re-hosting mod jars.** Packs reference Modrinth or CurseForge downloads.
- **Telemetry.** None. Outbound traffic is the site, Modrinth or CurseForge, and
  whatever Prism itself contacts.
