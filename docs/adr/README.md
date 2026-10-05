# Architecture decisions

One file per decision, numbered, never edited once accepted: a later decision
that changes one says so and links back. Each carries a status
(**accepted**, **proposed**, **superseded**), the context, the decision, and
what it costs.

The first seven follow the questions in `docs/HANDOFF.md` §3; 0008 was added
while building the scaffold, 0009 with the download site, 0010 with the header bar,
0011 with getting Prism for the player, 0012 with hiding the game window, 0013 with
the player's profile from Mojang.

| ADR | Decision | Status |
|---|---|---|
| [0001](0001-app-framework.md) | Wails v2 on Go 1.26 | accepted |
| [0002](0002-prism-data-root.md) | The user's own Prism root by default, a dedicated root as a setting | accepted |
| [0003](0003-pack-format.md) | packwiz as the pack source; installer for sync, `.mrpack` for a fresh install | accepted, unbuilt |
| [0004](0004-launcher-manifest.md) | A versioned `launcher.json`, embedded now, published by the site later | accepted |
| [0005](0005-server-status.md) | Server List Ping for every chapter with a server; Play stays live offline | accepted |
| [0006](0006-self-update-and-signing.md) | Manual updates and unsigned builds first | proposed |
| [0007](0007-target-oses.md) | Windows 11 and macOS on Apple Silicon | accepted |
| [0008](0008-design-tokens.md) | The launcher holds its own token source, seeded from the wiki; the wiki's era accents corrected | accepted |
| [0009](0009-download-site.md) | A static download site in `site/`, on Cloudflare Pages | accepted |
| [0010](0010-window-chrome.md) | The app draws its own header bar; our window buttons on Windows, native traffic lights on macOS | accepted |
| [0011](0011-getting-prism.md) | On approval, a verified, launcher-managed portable Prism for players without one | accepted |
| [0012](0012-hiding-the-game-window.md) | The launcher hides the game's window until the reload begins; the pack keeps its early window on | accepted |
| [0013](0013-mojang-profile.md) | The account page asks Mojang's public profile endpoints for the player's UUID and skin face; only the profile name leaves, nothing from the response but the UUID, name and face is kept | accepted |
