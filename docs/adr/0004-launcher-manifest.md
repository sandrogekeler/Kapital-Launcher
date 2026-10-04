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
