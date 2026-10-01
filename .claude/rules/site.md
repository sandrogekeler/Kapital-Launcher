---
paths:
  - "site/**"
---

# The download site

`site/README.md` covers the page, its links and the Cloudflare setup, and
`docs/adr/0009-download-site.md` records why it is built this way. The rules:

- **No script on the page.** The CSP in `public/_headers` is `default-src
  'none'` with styles, images and fonts from the site itself. Adding a script,
  an inline style, a data: URI or a remote asset means changing that policy
  on purpose, in the same change. The launcher window on the page cycles
  through its chapters on CSS animations alone (`src/showcase.css`), and its
  icons are lucide's paths inlined as SVG markup, which the policy allows.
- **The app's tokens and base styles, not the site's.** `src/style.css`
  imports `frontend/src/styles/tokens.css` and `frontend/src/styles/base.css`
  (faces, base rules, scrims), then `src/showcase.css`, the one thing the
  page draws that the app does not: the window's cycle, built from the app's
  motion tokens. A token change moves the app and the site together. The
  frontend style rules apply here: token utilities only, no literal colour,
  no pixel text size, no em dash. The suite's invariants cover
  `site/index.html` and `site/src`.
- **The window is a copy of the app's components.** Its markup carries the
  classes of `frontend/src/components` (HeaderBar, Sidebar, ChapterNav,
  ChapterButton, EngineCard, ChapterStage, Hero, ActionBar, Panels, Button,
  Pill, Fact) and the chapter facts of `data/launcher.json`. A change to one
  of those is a change to the site's copy, in the same pull request.
- **Links are data.** Download and GitHub live in `links.json`, the wiki in
  `data/launcher.json`. Never write a URL for one of the three buttons into
  `index.html`; the build fills them and rejects anything but https.
