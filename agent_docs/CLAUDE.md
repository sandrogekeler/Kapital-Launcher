# Kapital Launcher

A branded desktop front-end for the Kapitel Kapital Minecraft packs. It is not a
launcher: Prism Launcher is the engine (sign-in, Java, loaders, launching), and
this app finds it, shows the three chapters, and calls Prism's documented CLI.

## Stack

Wails v2 shell (Go 1.26, system WebView) with React 19, TypeScript, Vite and
Tailwind v4; Zustand for state; pnpm for the frontend, Go modules for the backend.
Same shape as Konnekt, deliberately (docs/adr/0001-app-framework.md).

## Where things live

- `design/tokens.json` is the one source of every design value. `pnpm gen:tokens`
  turns it into `frontend/src/styles/tokens.css` (Tailwind theme), `tokens.ts` and
  `backend/design/design_gen.go`. Never edit those three by hand.
- `data/launcher.json` is the chapter manifest built into the app, validated by
  `design/launcher.schema.json` and again by Go at startup. The site will publish
  the same shape later (ADR-4).
- `backend/models/` holds the data shapes; Wails generates `frontend/wailsjs/` from
  the bound methods on `App` in `app.go`. Regenerate with `wails generate module`.
- `backend/services/` holds everything with a side effect: Prism detection and
  launch, the server ping and its ticker, settings, logging, redaction. Each
  service ships with tests.
- `frontend/src/`: `components/` (view), `stores/` (one Zustand store per domain:
  chapters, engine, settings), `lib/` (pure helpers), `types/` (the data shapes).

## Rules

- **Go owns all side effects.** Process spawning, file I/O, URLs to the browser.
- **IPC via generated bindings only**: import from `wailsjs/go/`, never `window.go`.
  Every bound method returns `(T, error)`; `app_test.go` checks it.
- **A manifest is untrusted input.** It names things and never runs them: no
  command, JVM argument or path fields, https on allowlisted hosts only, refused
  whole on any violation (`services.ValidateManifest`).
- **Every value that reaches Prism is validated first**: instance id, server
  `host[:port]`, profile name, root path. `LaunchArgs` is the one place, tested.
- **One Zustand store per domain** (chapters, engine, settings, servers). Reads
  degrade without a bridge (`lib/ipc.ts`), writes revert and rethrow when a real
  backend rejects. Server status arrives as `server:status` events from Go's
  ticker; nothing in the frontend polls.
- **Styling is Tailwind utilities over the token layer.** No literal colour or
  pixel size in a component; a missing value is a token to add. Inline `style` is
  an ESLint error. The chapter accent is `data-chapter` on the root and
  `text-accent` everywhere else.
- **Icons come from `lib/icons.ts`**, the one module allowed to import
  `lucide-react`, rendered through `components/ui/Icon`.
- **No em dashes** in anything a user reads: UI copy, the manifest, README,
  commits, pull requests. A comma, a colon, a middle dot or two sentences.
- Go: `gofmt`, errors always handled (`//nolint:errcheck // reason` for a
  deliberate discard), `log/slog` for diagnostics, never `fmt.Println`.

## Commands

```bash
# frontend/
pnpm dev | build | typecheck | lint | test | format:check
pnpm test:coverage     # the floor, in vite.config.ts
pnpm check-tokens      # every token-named class compiles (builds if stale)
pnpm check-bundle      # entry chunk gzip budget
pnpm gen:tokens        # regenerate the three token outputs

# repo root
go vet ./... && go test ./...
wails dev | wails build | wails generate module
node scripts/validate-schemas.mjs
.claude/suite-check.py           # every gate above, from .claude/suite.json
```

**Definition of done:** `.claude/suite-check.py` is green (it is the same list CI
runs), the change is in scope for the current milestone in `ROADMAP.md`, and
anything security-relevant is checked against `SECURITY_CHECKLIST.md`. Track a
gap you cannot close under `HEALTH_CHECKLIST.md`'s Open backlog.

## Task tracking

GitHub Issues. `ROADMAP.md` holds direction and milestones; work items are
issues. `CONTRIBUTING.md` holds the rules CI enforces: one `type:` and one
`area:` label on every pull request (`pr-labelled`), a title in sentence case
with no em dash (`pr-copy`), the `type:` ladder that files each merged pull
request into the release notes. Branch from `origin/main`.

`docs/HANDOVER.md` is the state a session cannot derive: branches, open
questions, the issues still to file. Update it when handing over.

Vendored from `kollektiv-mc/Kollektiv` and never edited here: the three
`.claude/suite-*.py`, `.github/workflows/{pr-labelled,aislop,issue-priority}.yml`,
`.github/scripts/release-notes*.py`, `.github/release.yml`, `.aislop/base.yml`.

## Deeper rules, loaded on demand

`.claude/rules/` holds what matters in one part of the tree, scoped by `paths:`
so it costs no context until a matching file is opened: `frontend-style.md`,
`ipc.md`, `backend.md`, `tokens.md`, `manifest.md`, `builds-and-releases.md`,
`dependencies.md`, `aislop.md`.
