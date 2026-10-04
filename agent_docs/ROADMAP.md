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
- [x] Settings screen: Prism executable, Prism root, profile name, theme, and
      the developer's local pack address per chapter (#5)
- [x] Know whether a chapter's instance exists under the resolved root, and
      warn on the state line when it does not; Install replaces Play once
      milestone 4 can install (#24)

## Milestone 3: Launch end to end

- [ ] `prismlauncher [-d root] -l <id> [-s addr] [-a profile]` against a real
      instance, all three chapters
- [ ] Behaviour when Prism is already running (single-instance handling is
      Prism's; observe what `--launch` does then)
- [x] Open instance folder action, from a chapter's pen (#85, 2026-10-01)

## Milestone 4: Packs

- [x] Author the Frangfurd pack in packwiz; host `pack.toml` and `index.toml`:
      a Worker with static assets serves only the chapter folders of
      `kapital-packs` at `kapital-packs.alessandrogekeler.workers.dev` (#25,
      2026-10-01; what is public was decided on #35)
- [x] Fresh install: Install writes the `kapital-<id>` instance itself (#22,
      #24; `-I` cannot fix the folder name, ADR-2 amendment). Seen end to end
      on a clean new-player run (2026-09-30: managed Prism, Install, Play) and
      from the hosted pack headless (#101, 2026-10-01)
- [x] Day-to-day sync: pre-launch command built locally from a template,
      `"$INST_JAVA" -jar packwiz-installer-bootstrap.jar <pack.toml>`, never
      from the manifest (`preLaunchCommand` in `packinstance.go`, ADR-3;
      headless since #95)
- [x] Verify every download by hash; refuse a mismatch: packwiz-installer
      checks every indexed file against `index.toml` and refuses a changed one
      (seen 2026-09-30, the comment on #25); the two installer jars are pinned
      by size and SHA-256 (#22)
- [x] Update pack button and sync state line: "Update and play" in Play's place
      and a Version row when the synced pack.toml's hash differs from the
      source's (#71); the sync itself is still the pre-launch step
- [x] A chapter's own settings from the pen in its hero: memory and the JVM
      preset, written into its `instance.cfg` (#36, first half; ADR-2's second
      amendment)
- [x] Mods switched off from a chapter's settings: the manifest's quick toggles
      and an advanced list of every jar, kept in the launcher's settings and
      applied by the launcher's own copy as Prism's pre-launch command, which
      puts the disabled jars back for packwiz-installer and away again (issue
      156; ADR-2's ninth amendment)
- [x] Developer setting: override a chapter's `pack.toml` URL with a local
      `packwiz serve` address, so a pack is tried from a working copy first
      (`packOverrides` in settings.json, #41; edited on the settings screen
      since #5)
- [x] An installed chapter is switched between its published pack and the dev
      pack from its settings, in both directions, by rewriting the pack URL in
      its pre-launch command (`SetPackSource`; ADR-2's seventh amendment)

## Milestone 5: Manifest from the site

- [ ] `launcher.json` endpoint on the Kapitel Kapital Astro site, in the shape of
      `design/launcher.schema.json`
- [ ] Fetch with a unique User-Agent, cache, fall back to the bundled copy offline
- [x] The wiki's lore export (`/data/lore.json`) fetched the same way, cached in
      the app data dir, the bundled teaser as the fallback; the "From the wiki"
      panel picks a random page of the chapter's era on every switch (#58)
- [x] Screenshots from the wiki: listed by the lore export with the page each
      shows, downloaded and cached by Go and served from the app data dir, so
      the CSP stays as it is (#141); the bundled art is the offline fallback
- [x] The hero's picture and the "From the wiki" post cycle together, on every
      switch and every minute in view, a post linked to what the picture shows;
      off in the settings for each chapter's own picture (#142)

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

- [x] Changelog panel fed by the pack's own `changelog.json`, with the
      manifest's `changelog` as the fallback (issue 164, ADR-4 amendment)
- [x] Copy redacted log action (`services.Redactor`, #84)
- [x] Logs page per chapter: the instance's game logs and crash reports, newest
      first, opened masked and a chunk at a time, read only (#155, ADR-2 eighth
      amendment)
- [x] About section on the settings screen: version, disclaimer, licences (#87)
- [x] Forward frontend render errors to the Go log (#83)
- [x] Release workflow with attested artefacts for Windows and macOS; code
      signing decision (ADR-6): `.github/workflows/build.yml` (#107) builds both
      on every pull request and cuts an attested release from the Actions tab;
      the first prerelease dispatch is still to run
- [x] Create the labels in `.github/labels.yml` on the repository (`scripts/sync-labels.py`),
      done 2026-09-29
- [x] Vendor CodeQL and Scorecard from Konnekt (Kollektiv has neither), possible
      since the repository went public (2026-09-30)

## Download site

A static page separate from the wiki, with Download, GitHub and Wiki buttons
(ADR-9). Runs alongside the milestones; it waits on milestone 7 for a real
download.

- [x] `site/`: one page on the app's tokens, no script, links as data, a
      strict CSP; built in CI
- [x] The launcher window on the page, cycling through the three chapters on
      CSS animations, done 2026-10-01
- [x] Connect a Cloudflare Pages project to the repository (`site/README.md`,
      Hosting): `kapital-launcher` exists and deploys every pull request; its
      build watch paths are #29
- [x] GitHub link: the repository is public since 2026-09-30 (`site/links.json`,
      2026-10-02)
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
