# Kapital Launcher

A desktop launcher for the Kapitel Kapital Minecraft packs: Luxemburg,
Lichdenstein and Frangfurd. It is a branded front-end, not a launcher of its own.
[Prism Launcher](https://prismlauncher.org) does the heavy lifting (Microsoft
sign-in, Java, mod loaders, launching), and this app finds it, shows the three
chapters, and starts the right instance.

Personal use, for the people on the same servers.

## Getting started

Prerequisites: Go 1.26 or later, Node 22 or later with pnpm 11 or later
(`npm install -g pnpm`), the Wails CLI at the version `go.mod` names
(`go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0`; an older CLI
rewrites `go.mod`), and Prism Launcher installed and signed in. `wails doctor`
lists what your platform still needs.

```bash
cd frontend && pnpm install && cd ..
wails dev            # the app with hot reload
wails build          # build/bin/kapital-launcher
```

The frontend on its own, in a browser with no Go behind it:

```bash
cd frontend && pnpm dev
```

## Repository map

| Path | What it is |
|---|---|
| `design/tokens.json` | Every design value, once. Generated into CSS, TypeScript and Go by `pnpm gen:tokens` |
| `design/launcher.schema.json` | The shape of the chapter manifest |
| `data/launcher.json` | The manifest built into the app |
| `backend/` | Go: models, services (Prism, settings, logging, redaction), generated design values |
| `frontend/` | React, TypeScript, Vite, Tailwind |
| `docs/PACKS.md` | How a pack is authored, published, installed, synced and updated, in plain words |
| `docs/HANDOFF.md` | The brief this project was started from |
| `docs/HANDOVER.md` | Where things stand, for the next session |
| `docs/adr/` | Architecture decisions |
| `agent_docs/` | Conventions, roadmap, dependency and health and security checklists |

## Checks

```bash
.claude/suite-check.py    # every gate, from .claude/suite.json; the same list CI runs
```

## Code signing policy

Free code signing provided by [SignPath.io](https://about.signpath.io), certificate by [SignPath Foundation](https://signpath.org). This covers the Windows builds. The first beta was published before signing was set up and is not signed.

Every release is built from the public source by the project's release workflow on GitHub, and only those builds are signed.

- Committers and reviewers: [Alessandro Gekeler](https://github.com/sandrogekeler)
- Approvers: [Alessandro Gekeler](https://github.com/sandrogekeler)

Privacy policy: this program will not transfer any information to other networked systems unless specifically requested by the user or the person installing or operating it. It has no telemetry and no account of its own. To show what it shows, it reads the Kapitel Kapital wiki, asks each chapter's game server whether it is online, checks the pack host for a newer pack and GitHub for a newer Prism Launcher. It downloads Prism Launcher and a chapter's pack only when you ask it to. Signing in to Minecraft is done by Prism Launcher with Microsoft, under their own terms.

To remove it: on Windows, uninstall Kapital Launcher from Settings, Apps; on macOS, move it from Applications to the Trash. Your settings and worlds stay until you delete them yourself.

## Licences and notices

Kapital Launcher's source code is free software under the GNU General Public
License, version 3 (GPL-3.0-only); the text is in `LICENSE`. Copyright 2026
Alessandro Gekeler.

The licence covers the code. The Kapitel Kapital and Kapital Launcher names,
the logo and the chapter artwork in `frontend/src/assets/` are not under it
and stay the author's: a fork needs its own name and art.

Prism Launcher is GPL-3.0 and separately installed; this app calls its command
line and ships none of its code. Fonts: Inter, Fraunces and JetBrains Mono,
SIL OFL 1.1, licences in `frontend/src/assets/fonts/licences/`. Icons:
lucide, ISC.

NOT AN OFFICIAL MINECRAFT PRODUCT. NOT APPROVED BY OR ASSOCIATED WITH MOJANG
OR MICROSOFT.
