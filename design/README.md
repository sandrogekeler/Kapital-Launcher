# Design

Two data files and their schemas. Both are read by machines; neither is styled
code.

## `tokens.json`

The one place a design value is defined. `frontend/scripts/gen-tokens.mjs`
turns it into three outputs, and nothing else defines a colour, a size, a
radius or a duration:

```
design/tokens.json
        │  pnpm gen:tokens (from frontend/)
        ├──▶ frontend/src/styles/tokens.css    Tailwind @theme + themed custom properties
        ├──▶ frontend/src/styles/tokens.ts     the same values for code
        └──▶ backend/design/design_gen.go      window geometry for main.go
```

Values are plain data, never CSS: a colour is `{ hex, alpha? }` with a dark and
an optional light value (`null` inherits dark), an easing is four numbers, a
size is a bare number with the unit on its group. That is the shape Kollektiv's
`design/tokens.json` uses and the reason it is kept: a fourth consumer on a
different stack reads data, not someone else's stylesheet.

Naming is by **role**, never by appearance: `bg-raised`, not `grey-800`;
`chapter.luxemburg`, not `brass`. An appearance name becomes a lie the first
time a theme changes it.

The chapter accents are a group of their own, keyed by chapter id. The
generator emits `[data-chapter='<id>'] { --accent: ... }` for each, so the
whole UI switches colour from one attribute on the root and no component ever
names a chapter's colour.

Where the values come from, and why the wiki is not the file this app reads
directly: `docs/adr/0008-design-tokens.md`.

**Adding a token:** add it here by role, run `pnpm gen:tokens` from `frontend/`,
commit the three outputs with it. `pnpm check-tokens` then proves every
token-named class the source uses compiles to a rule. A missing value is a
token to add, never a literal to inline.

## `launcher.schema.json`

The shape of the chapter manifest: what the launcher shows and launches.
`data/launcher.json` is the copy built into the app, and the Kapitel Kapital
site is planned to publish the same shape (`docs/adr/0004-launcher-manifest.md`).

A manifest is untrusted input wherever it comes from, so the schema has no
field for a command, a JVM argument or a path, and `services.ValidateManifest`
in Go adds what a schema cannot say: chapter ids must be unique and have an
accent in `tokens.json`, and every URL must be https on an allowlisted host.

`node scripts/validate-schemas.mjs` checks both data files against their
schemas with no dependency; the Go tests check the manifest again.
