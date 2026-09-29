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
  on purpose, in the same change.
- **The app's tokens and base styles, not the site's.** `src/style.css`
  imports `frontend/src/styles/tokens.css` and `frontend/src/styles/base.css`
  (faces, base rules, scrims) and adds nothing. A token change moves the app
  and the site together. The frontend style rules apply here:
  token utilities only, no literal colour, no pixel text size, no em dash.
  The suite's invariants cover `site/index.html` and `site/src`.
- **Links are data.** Download and GitHub live in `links.json`, the wiki in
  `data/launcher.json`. Never write a URL for one of the three buttons into
  `index.html`; the build fills them and rejects anything but https.
