---
paths:
  - "site/**"
---

# The download site

`site/README.md` covers the page, its links and the Cloudflare setup, and
`docs/adr/0009-download-site.md` records why it is built this way. The rules:

- **One script, and it only picks the OS.** `src/detect.js` checks the
  download popup's Windows or macOS radio and does nothing else: no network,
  no storage. The CSP in `public/_headers` is `default-src 'none'` with
  scripts, styles, images and fonts from the site itself. Adding a second
  script, an inline script or style, a data: URI or a remote asset means
  changing that policy on purpose, in the same change. The launcher window on the page cycles
  through its chapters on CSS animations alone (`src/showcase.css`), and its
  icons are lucide's paths inlined as SVG markup, which the policy allows.
- **The app's tokens and base styles, not the site's.** `src/style.css`
  imports `frontend/src/styles/tokens.css` and `frontend/src/styles/base.css`
  (faces, base rules, scrims), then `src/popup.css` (the download popup's opening, closing and panel
  fade, on the motion tokens) and `src/showcase.css`, the one thing the
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
- **Links are data.** `links.json` holds `release` (a tag, or `null`) and
  GitHub; the page has no Wiki button since 2026-10-06. Never write a URL for the
  buttons or the popup's downloads into `index.html`; the build derives the
  installer and release notes URLs from the tag and fills them, and rejects a
  tag that is not `vX.Y.Z` or `vX.Y.Z-alpha.N`/`-beta.N` and anything but
  https.
