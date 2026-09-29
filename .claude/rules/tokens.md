---
paths:
  - "design/tokens.json"
  - "design/tokens.schema.json"
  - "frontend/scripts/gen-tokens.mjs"
  - "frontend/scripts/check-token-classes.mjs"
  - "frontend/src/styles/**"
  - "backend/design/**"
---

# The token pipeline

`design/tokens.json` is the source; `design/README.md` says why it is data and
not CSS. `frontend/scripts/gen-tokens.mjs` is the only reader and writes three
files that are committed and never edited:

| Output | Read by |
|---|---|
| `frontend/src/styles/tokens.css` | Tailwind (`@theme`), and hand-written CSS through `var()` |
| `frontend/src/styles/tokens.ts` | Code that needs a value: `CHAPTER_IDS`, `CHAPTER_ACCENTS`, `WINDOW`, `LAYOUT` |
| `backend/design/design_gen.go` | `main.go` for the window; `services.ValidateManifest` for `ChapterIDs` |

Run `pnpm gen:tokens` from `frontend/` after any change and commit all three.
The `generated` check in `.claude/suite.json` diffs them; CI runs it.

## Adding a value

1. Add it to `tokens.json` by **role**. A colour needs a `dark` value and a
   `light` one (or `null` to inherit). A chapter accent goes under
   `color.chapter` keyed by the chapter id; that key is what the manifest and
   `[data-chapter]` use.
2. Regenerate. If the value needs a new Tailwind namespace, teach
   `gen-tokens.mjs` to emit it and `check-token-classes.mjs` to look for it, in
   the same change: the checker builds candidate class names from the source
   and asserts each one used in `src/` compiles, and a namespace it does not
   know is a class it cannot vouch for.
3. Use it as a utility (`bg-<name>`, `text-<name>`, `rounded-<name>`,
   `tracking-<name>`) or as `var(--<name>)` in `styles/base.css` or
   `style.css`. `layout` and `effect` values are plain custom properties, not
   a Tailwind namespace: read them with the `var()` shorthand
   (`size-(--layout-icon-md)`, `brightness-(--effect-hover-brightness)`).

## Things the generator gets right on purpose

- Colours are `@theme inline` so `bg-canvas` compiles to `var(--bg)` and a theme
  or chapter switch retheme by changing the property. Everything else is plain
  `@theme` so the custom property exists for hand-written CSS too.
- Each duration is emitted twice: `--duration-*` for CSS and
  `--transition-duration-*`, which is the namespace Tailwind's `duration-*`
  utility actually reads.
- The type scale overrides Tailwind's own keys (`text-sm` is 13px here). A
  colour must never be named after a size key; the generator refuses one.
- Light values repeat under the `prefers-color-scheme` media query rather than
  being reordered, so an explicit `data-theme` always wins.
- `--font-display--font-variation-settings` and `--text-display--line-height`
  are Tailwind v4 companions: applying `font-display` or `text-display` applies
  them too, which is how Fraunces stays on its neutral axes.

## Provenance

The neutrals, hairlines, fonts, radii and motion are the Kapitel Kapital wiki's
(`src/styles/tokens.css` there); `fg-soft` and the chapter accents are the
handoff's. `docs/adr/0008-design-tokens.md` is the plan for the wiki to become
the file both sites and this app read.
