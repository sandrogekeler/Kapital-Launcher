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
- [x] Server List Ping for every chapter with a server, on a Go ticker, as
      events; the state line and facts panel read it (ADR-5)
- [x] The Kollektiv conventions: label taxonomy, issue forms with the priority
      question, the pull-request label gate, a title and em-dash gate, release
      notes generator, aislop gate, CONTRIBUTING and SECURITY

## Milestone 2: Detect Prism, for real

- [x] Confirm the standard install path on Windows 11 against a real install:
      `%LOCALAPPDATA%\Programs\PrismLauncher` (Prism 11.1.0, 2026-09-29)
- [ ] Confirm the standard install paths on macOS; the code marks each `[verify]`
- [x] Confirm `prismlauncher --version` prints to stdout on a Windows GUI build:
      it does, through a pipe, as `PrismLauncher 11.1.0` (2026-09-29)
- [x] A header bar in place of the OS title bar, holding the brand and the
      settings gear (ADR-10, #11)
- [x] Get Prism for a player who has none, on approval: a verified,
      launcher-managed portable copy with a one-click update offer (ADR-11, #23)
- [ ] Settings screen: Prism executable, Prism root, profile name, theme
- [x] Know whether a chapter's instance exists under the resolved root, and
      warn on the state line when it does not; Install replaces Play once
      milestone 4 can install (#24)

## Milestone 3: Launch end to end

- [ ] `prismlauncher [-d root] -l <id> [-s addr] [-a profile]` against a real
      instance, all three chapters
- [ ] Behaviour when Prism is already running (single-instance handling is
      Prism's; observe what `--launch` does then)
- [ ] Open instance folder action

## Milestone 4: Packs

- [ ] Author the Frangfurd pack in packwiz; host `pack.toml` and `index.toml`
- [ ] Fresh install: Install writes the `kapital-<id>` instance itself (#22,
      #24; `-I` cannot fix the folder name, ADR-2 amendment). Built; left is a
      real Prism opening it and the first Play installing the pack
- [ ] Day-to-day sync: pre-launch command built locally from a template,
      `"$INST_JAVA" -jar packwiz-installer-bootstrap.jar <pack.toml>`, never
      from the manifest
- [ ] Verify every download by hash; refuse a mismatch
- [ ] Update pack button and sync state line
- [x] Developer setting: override a chapter's `pack.toml` URL with a local
      `packwiz serve` address, so a pack is tried from a working copy first
      (`packOverrides` in settings.json, #41; a field on the settings screen
      comes with #5)

## Milestone 5: Manifest from the site

- [ ] `launcher.json` endpoint on the Kapitel Kapital Astro site, in the shape of
      `design/launcher.schema.json`
- [ ] Fetch with a unique User-Agent, cache, fall back to the bundled copy offline
- [ ] Screenshots by URL; CSP `img-src` grows by the site's host

## Milestone 6: Server status, the rest

The ping, the ticker and the state line shipped with the scaffold (ADR-5).
What is left:

- [x] Frangfurd's real address (`rails-enjoyed.tun.ply.gg`, 2026-09-29), with the
      ping following the host's SRV record for the port
- [ ] Lichdenstein's real address
- [ ] Show the MOTD and the player sample somewhere the reference has room for
- [ ] Drop the per-minute "connection refused" log line to debug once the
      addresses are real

## Milestone 7: Polish

- [ ] Changelog panel fed by the manifest
- [ ] Copy redacted log action (`services.Redactor`)
- [ ] About screen: version, disclaimer, licences (fonts, lucide, Prism's GPL notice)
- [ ] Forward frontend render errors to the Go log
- [ ] Release workflow with attested artefacts for Windows and macOS; code
      signing decision (ADR-6)
- [x] Create the labels in `.github/labels.yml` on the repository (`scripts/sync-labels.py`),
      done 2026-09-29

## Download site

A static page separate from the wiki, with Download, GitHub and Wiki buttons
(ADR-9). Runs alongside the milestones; it waits on milestone 7 for a real
download.

- [x] `site/`: one page on the app's tokens, no script, links as data, a
      strict CSP; built in CI
- [ ] Connect a Cloudflare Pages project to the repository (`site/README.md`, Hosting)
- [ ] GitHub link, once the repository is public
- [ ] Download link, once the release workflow publishes a release
- [ ] A custom domain, if one is wanted

## Out of scope, and why

- **Handling Microsoft sign-in.** Needs an approved Azure app id that solo
  developers are refused. Prism has one. (HANDOFF §1)
- **Bundling or modifying Prism.** GPL-3.0 duties, and a fork cannot reuse
  Prism's client id. The app points at prismlauncher.org instead. (HANDOFF §5)
- **Re-hosting mod jars.** Packs reference Modrinth or CurseForge downloads.
- **Telemetry.** None. Outbound traffic is the site, Modrinth or CurseForge, and
  whatever Prism itself contacts.
