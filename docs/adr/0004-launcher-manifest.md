# ADR-0004: Where launcher metadata comes from

**Status:** accepted, 2026-09-28. The shape is built; the endpoint is not
(Roadmap, milestone 5).

## Context

The chapter list, blurbs, pack facts, server address, changelog and wiki
teasers need a source. The handoff (§3, ADR-4) recommended a `launcher.json`
the Kapitel Kapital Astro site publishes at build time, so there is one source
of truth and the wiki's own text is reused.

## Decision

- **The shape is decided now**, in `design/launcher.schema.json`, versioned
  (`version: 1`). `data/launcher.json` is the copy built into the app and
  validated at startup; it is what the app shows today and the offline
  fallback later.
- **The site publishes the same shape** at a fixed path once milestone 5
  arrives. The app fetches it over https from `kapitel-kapital.pages.dev`
  only, with a unique User-Agent, a timeout and a size bound, parses it with
  the same `ParseManifest`, and falls back to the bundled copy on any failure.
- **A manifest names things and never runs them.** No field for a command, an
  argument or a path, and `DisallowUnknownFields` on parse, so a field this
  build does not know is refused rather than ignored.
- Screenshots are bundled today; by URL from the site later, at which point
  the CSP's `img-src` grows by that one host.

## Consequences

- The site gains a build step that emits the manifest from the wiki's
  taxonomy and world pages; the blurbs there are lore, the launcher's are
  gameplay, so the site's emitter needs a place for the second kind (a
  `launcher:` block in each world's frontmatter, say). That is a wiki change
  and is not made from this repo.
- Signing the manifest is optional and deferred: the transport is https to
  the author's own host, and the manifest cannot express anything executable.

## Amendment, 2026-10-04: a server names every address

Frangfurd's server answers at two addresses, and the game's own server list
carries both, so the manifest's one `server.address` could not say which a
player uses (issue 151). The shape, still `version: 1` because the one shipped
copy is the bundled file:

- `server.address` is replaced by **`server.addresses`**, a list of
  `{ label, address }`, at least one, the **first the default**. Replacing,
  not adding beside, keeps one source of truth: a chapter with one address is
  a list of one, and the wiki's emitter writes the same list whatever the
  count. An older `address` field is refused like any field this build does
  not know (`DisallowUnknownFields`).
- Every address is held to the one `host[:port]` rule (`ParseServerAddress`) in
  the schema and in `services.ValidateManifest`, in any position; a label is 1
  to 24 letters, digits, spaces, dots and hyphens, and unique within the
  chapter (case-insensitively). A manifest with a bad one is refused whole.
- The player's choice is not in the manifest. It is a **label** saved per
  chapter in the app's settings (`serverChoices`), checked against that
  chapter's own list on save, and resolved by `services.ServerAddress`: the
  chosen label's address, else the first when the choice is missing or no
  longer listed. The ping, its ticker and Play's `--server` all use it, and
  `LaunchArgs` still validates what reaches Prism. A manifest still names
  things and runs nothing.

## Amendment, 2026-10-04: joining is the player's switch, not the manifest's

The manifest's `server.joinOnLaunch` said whether Play joined a chapter's
server, which made Lichdenstein's button read "Join Lichdenstein" for every
player whether or not they wanted to be dropped into a server (issue 163). It
is **retired**, not kept as a default:

- `joinOnLaunch` is removed from `models.Server`, the schema (the property and
  its `required` entry), the bundled `data/launcher.json` and the TypeScript
  types. A manifest that still carries it is refused whole like any field this
  build does not know (`DisallowUnknownFields`), so the wiki's emitter must not
  write it. Keeping it as the default for a player who never set the switch
  would have let a manifest decide that a game starts by connecting to a
  server, which is the one thing a manifest should not decide.
- Whether Play joins is **the player's switch**, saved per chapter in the app's
  settings as `joinServers`: the ids of the chapters whose server is joined.
  Off for every chapter until turned on. The ids are checked against the
  manifest on save (`ValidateJoinServers`: a chapter that has a server, never
  an address) and pruned on read like `serverChoices`.
- The switch lives with the address choice, on each chapter's own settings
  page, which gains a Server section for a chapter with a server; the app's
  Settings screen loses its Server section. The address joined is still the one
  `services.ServerAddress` resolves, and `LaunchArgs` still validates what
  reaches Prism's `--server`.
- Play reads "Join <chapter>" only while the switch is on, and "Play <chapter>"
  otherwise. Frangfurd and Luxemburg are no different from Lichdenstein now.

## Amendment, 2026-10-04: the changelog comes from the pack, the manifest's is the fallback

Changelog entries typed into `data/launcher.json` would ship with a launcher
release, and all three chapters' were empty (issue 164). A chapter's changelog
is now a file its pack's host serves beside `pack.toml`
(`<chapter>/changelog.json`, docs/PACKS.md), read by the launcher from the pack
source it already trusts for that chapter, under that source's URL rule, with
`pack.toml`'s timeout and a size bound, and validated strictly
(`services.ParseChangelog`): a file that deviates is refused whole.

The manifest's `changelog` stays, in the schema and the Go and TypeScript
shapes, as the **fallback**: it is what the panel shows for a chapter whose
pack publishes no changelog (a 404) or whose file could not be read. It is
still data a manifest names and never runs: one summary per version, shown as
plain text.
