---
paths:
  - "data/**"
  - "design/launcher.schema.json"
  - "backend/services/manifest.go"
  - "backend/models/chapter.go"
  - "frontend/src/lib/manifest.ts"
---

# The launcher manifest

`data/launcher.json` is what the app shows and launches. Three things describe
its shape and all three move together: `design/launcher.schema.json` (editor
time, `node scripts/validate-schemas.mjs`), `backend/models/chapter.go` (the Go
struct, with `DisallowUnknownFields` on parse) and
`services.ValidateManifest` (the rules a schema cannot state). A field added to
one is added to the other two in the same change, and to
`frontend/src/types/index.ts`.

## What a manifest may and may not say

It names things. It carries no command, JVM argument or filesystem path, and
never will: a manifest is untrusted input wherever it comes from
(`agent_docs/SECURITY_CHECKLIST.md`, S2). Pre-launch commands for pack sync
are built in Go from a template and the manifest contributes
only the `pack.toml` URL.

Every URL is https on a host in `AllowedManifestHosts`. Adding a host is a
deliberate edit to that list with a reason in the commit. The one exception is a
chapter's `map` (below): a playit tunnel serves BlueMap over http.

## Chapters

- `id` is lowercase, unique, and must have an accent under `color.chapter` in
  `design/tokens.json`; that is how `[data-chapter]` finds its colour.
- `instance.id` is the Prism instance folder name, `kapital-<id>` by convention
  (ADR-2), and is what `--launch` receives.
- `server` is `null` or `{ addresses, software }`. `addresses`
  is a list of `{ label, address }`, at least one, the first the default
  (ADR-4, amendment). Each address is `host[:port]`, any of them pinged for
  the status line; the label is short, unique in the chapter and is what the
  player's choice in the chapter's settings is saved as, never the address. The one in use
  is `services.ServerAddress(chapter, settings.ServerChoices)`: use it, never
  `Addresses[0]`. Whether Play joins the server is not the manifest's to say:
  `joinOnLaunch` is retired (ADR-4, second amendment), a manifest that carries
  it is refused, and the player's switch (`settings.JoinServers`, on the
  chapter's settings page) is what turns Play into Join and adds `--server` to
  the launch (`services.JoinsServer`). `software` is a fact for the panel.
- `pack.toggles` is optional: up to twelve `{ name, jarPrefix }`, the mods the
  chapter's settings offer as quick switches (issue 156). `jarPrefix` is the part
  of the jar's file name before its version, separator included
  (`DistantHorizons-`, `colorwheel-neoforge-`, `colorwheel_patcher-neoforge-`):
  every jar in the instance's mods folder that starts with it is that mod, so the
  switch survives a version bump. Take it from the `filename` of the mod's
  `.pw.toml` in `kapital-packs`. It is a prefix and never a path or a pattern
  (`[A-Za-z0-9][A-Za-z0-9._+-]{2,63}`), names are unique, and no prefix starts
  another, so a jar is never two switches (`validateToggles`).
- `map` is optional: a chapter's BlueMap web map (issue 161, ADR-4's third
  amendment), `http` or `https` on `*.tun.ply.gg` with an explicit port, lowercase
  and bare (no user info, path, query or fragment); `services.checkMapURL` refuses
  anything else, and the rest of the manifest keeps the https rule. The build
  copies exactly these origins into `index.html`'s `frame-src`
  (`vite.config.ts`, `pnpm check-csp`), so a new map address is a build away from
  being framed. A chapter with none leaves the field out.
- `state` is `released`, `development` or `planned`.
- An unsettled fact is `"[PLACEHOLDER]"` for a string and `null` for a number or
  URL. The UI renders both faint; do not invent a value to fill a slot.
- `wiki.path` is relative to `wiki.baseUrl`, and the wiki's own slug rules
  apply: `/wiki/locations/bellum-castle`, `/worlds/lichdenstein`.

## Where it is read

`main.go` embeds it and `NewApp` parses it, so a bad manifest fails at startup.
The frontend imports the same file (`lib/manifest.ts`) as the initial state and
the browser-only fallback; `GetManifest` replaces it once Go answers.
