# Download site

One static page: what the launcher is, and Download, GitHub and Wiki buttons.
It is separate from the Kapitel Kapital wiki and runs no script. The decision
record is `docs/adr/0009-download-site.md`.

## Where the look comes from

Nothing visual is defined here. `src/style.css` imports the app's generated
`frontend/src/styles/tokens.css` and its shared `frontend/src/styles/base.css`
(fonts, base rules, scrims), and `index.html` uses the
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
pnpm preview    # build, then serve dist/ locally
```

There is no `vite` dev server script on purpose: the page's fonts and images
live in `frontend/`, outside the dev server's root, and only the build
resolves them. A build takes well under a second. `pnpm preview` does not
apply `public/_headers`; only Cloudflare does.

## Hosting

Cloudflare Pages, connected to this repository from the dashboard. There is no
Wrangler file: the settings below live in the Pages project.
`public/_headers` sets the CSP and caching and is copied into `dist/`.

In the Cloudflare dashboard under Workers & Pages, create a Pages project by
connecting this repository, then set:

| Setting | Value |
|---|---|
| Framework preset | None |
| Production branch | `main` |
| Root directory | `site` |
| Build command | `pnpm build` |
| Build output directory | `dist`, which is relative to the root directory. Not `site`: that is the unbuilt source, whose links are empty and whose fonts and images point into `frontend/`. |
| Environment variable | `PNPM_VERSION` = `12`, the version CI uses. The build image's default, 10.11.1, installs the site but predates `minimumReleaseAge` in `pnpm-workspace.yaml`, so the release-age policy would not apply. |
| Build watch paths | Include `site/*`, `frontend/src/styles/*`, `frontend/src/assets/*`, `data/launcher.json`. Paths are from the repository root. |

Pages installs the dependencies itself before the build command, from the root
directory's `pnpm-lock.yaml`. Every other branch gets a preview deployment.

These settings come from Cloudflare's Pages documentation as read on
2026-09-28 and have not been run on Cloudflare yet. The build was checked
locally, and the install with pnpm 12 and with 10.11.1.
