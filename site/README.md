# Download site

One static page: what the launcher is, and Download, GitHub and Wiki buttons.
It is separate from the Kapitel Kapital wiki and runs no script. The decision
record is `docs/adr/0009-download-site.md`.

## Where the look comes from

Nothing visual is defined here. `src/style.css` imports the app's generated
`frontend/src/styles/tokens.css` and its fonts, and `index.html` uses the
wordmark and a screenshot from `frontend/src/assets/`. The buttons carry the
classes of the app's `Button` variants: `play` for Download, `ghost` for the
others. A value the page needs and the tokens do not have is a token to add in
`design/tokens.json`, the same rule as the app.

## The links

| Button | Source | When `null` |
|---|---|---|
| Download | `links.json` `download` | Shows "Download · coming soon", not a link |
| GitHub | `links.json` `github` | Left out |
| Wiki | `data/launcher.json` `wiki.baseUrl` | Never null; the build fails |

`vite.config.js` fills them in at build time and refuses anything that is not
https. Both start `null`: the repository is private and has no release yet.
When they exist, set:

```json
{
  "download": "https://github.com/sandrogekeler/Kapital-Launcher/releases/latest",
  "github": "https://github.com/sandrogekeler/Kapital-Launcher"
}
```

## Commands

```bash
pnpm install
pnpm build      # dist/
pnpm preview    # build, then serve dist/ with wrangler dev, _headers applied
```

There is no `vite` dev server script on purpose: the page's fonts and images
live in `frontend/`, outside the dev server's root, and only the build
resolves them. A build takes well under a second.

## Hosting

Workers static assets (`wrangler.jsonc`): no Worker script, only `dist/`.
`public/_headers` sets the CSP and caching and is copied into `dist/`.

To connect it once, in the Cloudflare dashboard under Workers & Pages, create
an application by importing this repository and set:

| Setting | Value |
|---|---|
| Project name | `kapital-launcher`, matching `name` in `wrangler.jsonc` |
| Root directory | `site` |
| Build command | `pnpm build` |
| Deploy command | the default, `npx wrangler deploy` (Wrangler from `package.json`) |
| Build variable | `PNPM_VERSION` = `12`, the version CI uses. The build image's default pnpm predates `minimumReleaseAge` in `pnpm-workspace.yaml`. |
| Build watch paths | `site/*`, `frontend/src/styles/*`, `frontend/src/assets/*`, `data/launcher.json` |

These settings come from Cloudflare's documentation as read on 2026-09-28 and
have not been run on Cloudflare yet. The build and the headers were checked
locally with `pnpm preview`.
