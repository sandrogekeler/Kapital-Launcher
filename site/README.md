# Download site

One static page: what the launcher is, the Download, GitHub and Wiki buttons,
and under them the launcher window itself, cycling through its three
chapters. It is separate from the Kapitel Kapital wiki and runs no script.
The decision record is `docs/adr/0009-download-site.md`.

## Where the look comes from

Nothing visual is defined here. `src/style.css` imports the app's generated
`frontend/src/styles/tokens.css` and its shared `frontend/src/styles/base.css`
(fonts, base rules, scrims), and `index.html` uses the
Kapital Launcher logo (the author's two-line artwork, scaled to 800px wide)
and a screenshot from `frontend/src/assets/`. The logo is the page's `h1`,
its alt text the heading. The buttons carry the
classes of the app's `Button` variants: `play` for Download, `ghost` for the
others. A value the page needs and the tokens do not have is a token to add in
`design/tokens.json`, the same rule as the app.

The page is dark whatever the visitor's system preference (`data-theme="dark"`
on the root): the artwork and the window are drawn for the dark palette.

## The window

Below the buttons is the launcher window at its real size
(`layout.window` in the tokens), drawn with the classes of the app's own
components (`frontend/src/components`): the header bar, the chapter nav with
its gliding highlight, the account card, and the chapter card with its hero,
action bar and panels, filled from `data/launcher.json`. The icons are
lucide's paths inlined as SVG, the same glyphs the app renders. The window
is a picture: nothing in it is a control, it is hidden from assistive
technology, and a sentence before it names the three chapters.

It cycles through the chapters on CSS animations alone, in `src/showcase.css`,
with the app's own motion: the card slides the way the content moves and
blurs at the middle of its travel (`frontend/src/style.css`), the nav
highlight glides to the next row in the same duration and easing, the row's
art fades in behind it, and the glow behind the window and the Download
button take the open chapter's colour. A chapter dwells for
`motion.duration.dwell`, then the next slides in over `motion.duration.slow`;
the keyframe stops in `showcase.css` are fractions of that cycle and say how
to recompute them if either duration changes. With reduced motion the cards
cross-fade on the same clock and the highlight steps instead of gliding.

Narrower than the app's minimum window (1024px) the panels and the window's
disclaimer are left out and the window takes the height of its content;
narrower than 768px the sidebar and the hero's tools go too, and the card
alone is shown.

A change to one of the components the window copies, or to the manifest's
chapter facts, is a change to this page in the same pull request.

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
