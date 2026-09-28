# ADR-0008: Where the design tokens live

**Status:** accepted, 2026-09-28. The accent question below was answered the same day.

## Context

The brand source is the Kapitel Kapital wiki: its `src/styles/tokens.css`
holds the neutrals, hairlines, fonts, type scale, radii and motion, and its
`src/lib/taxonomy.mjs` maps each era to an accent. That file is CSS, written
for one Astro site, and the wiki is both its producer and its only consumer.

Kollektiv's rule for its suite is that a token set is data in one place,
vendored into each product, and nothing both produces and consumes it. This
launcher is a second consumer of the Kapitel Kapital brand, so the same
question arrives: what does it read?

## Decision

- **`design/tokens.json` in this repo is the source this app builds from**,
  in Kollektiv's tech-neutral shape, seeded from the wiki's values on
  2026-09-28 and recorded as such in its `source` block. `pnpm gen:tokens`
  derives the CSS, TypeScript and Go from it.
- **The wiki stays the brand's source of record.** A value that changes there
  is changed here by hand until the wiki publishes its tokens as data. That
  is the intended direction: the wiki emits `tokens.json` in this shape and
  generates its own `tokens.css` from it, and this repo vendors the file the
  way Konnekt vendors Kollektiv's. It is a wiki change and is not made from
  this repo.
- **Chapter accents are the handoff's**, which §2 calls canonical: Luxemburg
  `#e0aa4e`, Lichdenstein `#5cbfa2`, Frangfurd `#7cbfe0`. They are one group
  in the token file, so flipping them is one edit.

## The wiki's accents

The handoff's §9 asked for a "side fix" in the wiki: Lichdenstein from `ice`
to `verdigris`, Frangfurd from `oxide` to `ice`. The author confirmed the
eras are gold, green and blue and that the wiki's mapping was the error.
**The wiki was corrected on 2026-09-28** on the same branch as this repo,
and not as the one-line change the handoff described, because of what was
read from the wiki that day:

- `tokens.css` states the era mapping in prose (brass, ice, oxide) and
  assigns `verdigris` to events and concepts and `ice` to locations as *type*
  colours, with the reasoning written beside them. Recolouring two eras
  collides with that scheme.
- `LocationTree.astro` and `graph.astro` hard-code the three accent names in
  their CSS, and `tokens.css` has no `[data-accent='verdigris']` rule. Changing
  `taxonomy.mjs` alone breaks the era accent on those pages.

So the fix touched `taxonomy.mjs`, `tokens.css` (the accent rule, a
`verdigris-bright` tint for both themes, and the prose that stated the old
mapping), the two components, the contrast check and `PLAN.md`. The type
colours (location ice, event and concept verdigris) were left as they are:
they are a separate axis that happens to share two hues with the eras, and
`taxonomy.mjs` now says so. Both sites agree with the launcher's
`color.chapter` group.
