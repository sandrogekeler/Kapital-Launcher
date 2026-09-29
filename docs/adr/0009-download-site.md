# ADR-0009: The download site

**Status:** accepted, 2026-09-28. Built, not yet deployed.

## Context

The launcher needs a public page with three ways out: download it, read its
source, read the wiki. The author asked for it separate from the Kapitel
Kapital wiki, kept simple, and hosted on Cloudflare Pages.

Three facts read on 2026-09-28 shaped the answer:

- Cloudflare's own documentation tells new projects to use Workers static
  assets instead of Pages: "If you are starting a new project, use Workers
  instead of Pages. Pages continues to work, but new features and
  optimizations are focused on Workers"
  (developers.cloudflare.com/workers/best-practices/workers-best-practices).
  New Pages projects can still be created, and the wiki is one.
- The repository is private and has no release, so a GitHub link shows a
  visitor a 404 and a Download link has nothing to point at.
- Everything the page looks like already exists here: the generated
  `tokens.css`, the fonts, the wordmark, a screenshot and the `Button`
  variants.

## Decision

- **`site/` in this repository**, its own pnpm project beside `frontend/`.
  It imports `frontend/src/styles/tokens.css`, the fonts and the images by
  relative path rather than copying them, so the page and the app cannot
  drift apart. Since 2026-09-29 (#16) it also imports the shared
  `frontend/src/styles/base.css` (faces, base rules, scrims) instead of
  keeping its own copy, and adds no style of its own.
- **Vite and Tailwind v4, no framework, no script.** One HTML page styled with
  the same token utilities as the app. The built output is HTML, one CSS file,
  fonts and two images.
- **Cloudflare Pages**, connected from the dashboard: root directory `site`,
  build command `pnpm build`, output directory `dist`. The author chose it
  over Workers static assets, having weighed Cloudflare's guidance above: it
  is what the wiki runs on, and a static page needs nothing Workers adds.
  There is no Wrangler file and no Wrangler dependency; the project's
  settings live in the dashboard and are written down in `site/README.md`.
- **The links are data.** `site/links.json` holds Download and GitHub as an
  https URL or `null`; the wiki comes from `data/launcher.json`'s
  `wiki.baseUrl`. A build-time Vite plugin fills them in: a null Download
  shows "coming soon", a null GitHub is left out, and a non-https URL fails
  the build. Both start null.
- **A strict CSP in `site/public/_headers`**: `default-src 'none'` with
  styles, images and fonts from the site itself. Possible only because the
  page has no script and inlines no asset.

## Consequences

- The site depends on files inside `frontend/`. Moving the token output, the
  fonts or the screenshots breaks the site build, which CI runs in its own
  job for that reason.
- Deploys are Pages' own builds. CI builds the site but does not deploy it.
  Every other branch gets a Pages preview deployment.
- Pages is the platform Cloudflare no longer develops. If it is ever retired
  or the site needs something only Workers has, the move is a `wrangler.jsonc`
  with `assets.directory` set to `./dist`
  (developers.cloudflare.com/workers/static-assets/migration-guides/migrate-from-pages);
  the page itself does not change.
- Until a custom domain is added the site lives on `<project>.pages.dev`.
- Download and GitHub go live by editing one file, when milestone 7's
  release workflow exists and the repository is public.
