# Kapital Launcher: Dependency Policy and Inventory

A decision record, not a lockfile mirror: one line of rationale per direct
dependency, and what was considered and not added. `.claude/rules/dependencies.md`
points here before a dependency is added; record the addition in the same pull
request.

## Policy

**Go:** prefer the standard library. This app runs on the user's machine and
spawns a process on their behalf, so every dependency is something trusted with
that. Today there is exactly one.

**npm:** prefer what is already in the tree. Anything heavy or rarely used is
lazy-loaded with `React.lazy`, and `pnpm check-bundle` holds the entry chunk to
its budget.

## System dependencies

Wails links against the host WebView: WebView2 on Windows (present on Windows
10/11), WebKit on macOS, and `libgtk-3-dev` plus `libwebkit2gtk-4.1-dev` on
Linux. `wails doctor` lists what a machine is missing. None is a Go or npm
package.

## Go (`go.mod`, direct)

| Module | Rationale |
|---|---|
| `github.com/wailsapp/wails/v2` | The app shell: Go to WebView bridge, binding generation. Pinned by hand, not bumped by Dependabot: a minor changes the generated bindings and the webview it links against. |

| `golang.org/x/sys` | `windows.WinVerifyTrustEx`, Windows' own Authenticode check for the managed Prism's executable (ADR-11), called directly so no shell or external tool is involved. Already in the build through Wails; now direct. |

Everything else in `go.mod` is `// indirect`, pulled in by Wails.

**Considered and not added:** a JSON Schema validator for the manifest. The Go
side needs rules a schema cannot express anyway (host allowlist, chapter ids
known to the token set), so `services.ValidateManifest` states them all in
Go, and `scripts/validate-schemas.mjs` checks the JSON against the schema at
edit time with no dependency either. Revisit if the manifest grows past what a
hundred lines of Go read comfortably.

## Frontend (`frontend/package.json`, direct)

| Package | Rationale |
|---|---|
| `react`, `react-dom` | UI framework |
| `zustand` | One store per domain (chapters, engine, settings) |
| `lucide-react` | The icon set, reached only through `src/lib/icons.ts`. ISC. Tree-shakes to the icons re-exported there. |

Dev tooling (Vite, TypeScript, ESLint, Prettier, Vitest, Tailwind, Testing
Library) is inspectable in `devDependencies` and does not ship.

Two pins worth a line: `vitest` and `@vitest/coverage-v8` must move together,
so Dependabot ignores both; and `typescript` stays on 6.x until
`typescript-eslint` declares support for 7.

## Download site (`site/package.json`, dev only)

Nothing here ships as code: the built site is HTML, CSS, fonts and images.

| Package | Rationale |
|---|---|
| `vite`, `tailwindcss`, `@tailwindcss/vite` | The same build and styling as the frontend, same versions, so the page uses the app's token utilities |

**Considered and not added:** Astro, which the wiki uses. One section with
three links needs no framework, and a plain Vite page reuses the app's
toolchain. Wrangler, Cloudflare's CLI: Pages builds and deploys from the
dashboard's Git connection, so nothing here calls it.

## Fonts

Inter, Fraunces and JetBrains Mono, copied from the wiki's `public/fonts/`,
all SIL OFL 1.1; each licence sits beside the files in
`frontend/src/assets/fonts/licences/`. Not npm packages, so listed here.
