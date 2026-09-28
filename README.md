# Kapital Launcher

A desktop launcher for the Kapitel Kapital Minecraft packs: Luxemburg,
Lichdenstein and Frangfurd. It is a branded front-end, not a launcher of its own.
[Prism Launcher](https://prismlauncher.org) does the heavy lifting (Microsoft
sign-in, Java, mod loaders, launching), and this app finds it, shows the three
chapters, and starts the right instance.

Personal use, for the people on the same servers.

## Getting started

Prerequisites: Go 1.25, Node 22 with pnpm, the Wails CLI
(`go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0`), and Prism
Launcher installed and signed in. `wails doctor` lists what your platform
still needs.

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
| `docs/HANDOFF.md` | The brief this project was started from |
| `docs/adr/` | Architecture decisions |
| `agent_docs/` | Conventions, roadmap, dependency and health and security checklists |

## Checks

```bash
.claude/suite-check.py    # every gate, from .claude/suite.json; the same list CI runs
```

## Licences and notices

Prism Launcher is GPL-3.0 and separately installed; this app calls its command
line and ships none of its code. Fonts: Inter, Fraunces and JetBrains Mono,
SIL OFL 1.1, licences in `frontend/src/assets/fonts/licences/`. Icons:
lucide, ISC.

NOT AN OFFICIAL MINECRAFT PRODUCT. NOT APPROVED BY OR ASSOCIATED WITH MOJANG
OR MICROSOFT.
