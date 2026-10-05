# Download site

One static page: what the launcher is, the Download and GitHub buttons,
and under them the launcher window itself, cycling through its three
chapters. It is separate from the Kapitel Kapital wiki and runs one small
script, which only picks the operating system in the download popup.
The decision record is `docs/adr/0009-download-site.md`.

## Where the look comes from

Nothing visual is defined here. `src/style.css` imports the app's generated
`frontend/src/styles/tokens.css` and its shared `frontend/src/styles/base.css`
(fonts, base rules, scrims), and `index.html` uses the
Kapital Launcher logo (the author's one-line artwork, the file the app's header bar draws, 560px wide)
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
| Download | `links.json` `release`, a tag | Shows "Download · coming soon", no popup |
| GitHub | `links.json` `github` | Left out |

`vite.config.js` fills them in at build time and refuses anything that is not
https. `release` is a tag such as `v1.0.0-beta.1`, checked against
`^v\d+\.\d+\.\d+(-(alpha|beta)\.\d+)?$`; from it and the `github` link the
build derives the two installers and the release notes:

- `<github>/releases/download/<tag>/Kapital-Launcher-<tag>-windows-amd64-setup.exe`
- `<github>/releases/download/<tag>/Kapital-Launcher-<tag>-macos-universal.dmg`
- `<github>/releases/tag/<tag>`

The tag is bumped by hand for each release, because `releases/latest/download/`
skips prereleases. It stays `null` until there is a release to point at; GitHub
was set on 2026-10-02, the repository being public. When a release exists, set:

```json
{
  "release": "v1.0.0-beta.1",
  "github": "https://github.com/sandrogekeler/Kapital-Launcher"
}
```

## The download popup

With a release set, Download is a button that opens a native popover
(`popovertarget`), no script needed: a heading, a Windows or macOS switch, the
one download for the chosen system, a line on what to expect, a note that the
beta is not code-signed yet with a link to the release notes, and a close
button. The switch is a fieldset of two radios and CSS `:has()`, so it works
without script. Windows is checked by default.

`src/detect.js` is the page's one script, loaded as a module and emitted by
Vite as a file under `assets/`. It checks the radio for the visitor's system
(`navigator.userAgentData?.platform`, then `navigator.platform`, then the user
agent) and does nothing else: no network, no storage. iPhone, iPad, Android and
Linux are neither, and keep Windows checked. With `release` set to `null`, the
build leaves the popup out of the page.

Its heading is in the display face, like the wiki's; the tag sits above it,
filled in at build time. It opens and closes with the app's reveal (a short
rise out of a slight blur) while the page dims and softens behind it, and the
other system's panel fades in when the switch moves: `src/popup.css`, on the
motion tokens, with no script. A browser without `@starting-style` shows and
hides it plainly, and reduced motion turns the transitions off.

The CSP in `public/_headers` allows it with `script-src 'self'` and nothing
more, so an inline script would be blocked.

## The code signing policy

Above the footer is one quiet line, "Code signing policy", a `details`
element that opens without script. It holds the section SignPath Foundation
asks a project to publish on its download page (signpath.org/terms, issue
179): the attribution sentence, the two team roles, the privacy sentence, and
how to remove the app. The heading and those sentences are SignPath's wording
and stay as they are. The same text is in the repository's `README.md` and in
`.github/release-body.md`, which heads every release; change the three
together.

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
