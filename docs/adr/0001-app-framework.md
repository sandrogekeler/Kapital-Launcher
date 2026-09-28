# ADR-0001: App framework

**Status:** accepted, 2026-09-28

## Context

The handoff weighed Wails v2, Wails v3 and Tauri (§3, ADR-1). Checked on
2026-09-28: the latest Wails v2 release is v2.16.0 (2026-09-14) and its
`go.mod` requires Go 1.25; Wails v3 is at v3.0.0-beta.26 (2026-09-25) and its
status page still calls it beta. Konnekt, the author's other desktop app, is
on Wails v2.

## Decision

Wails v2.16.0 on Go 1.25, with React 19, TypeScript, Vite and Tailwind v4 in
the frontend and Zustand for state: the Konnekt stack, so its conventions,
its token pipeline shape and its agent tooling carry over with the names
changed.

The window geometry comes from `design/tokens.json` through the generated
`backend/design/design_gen.go`, so the shell opens the window the layout was
drawn for.

## Consequences

- One Go dependency. Wails is pinned by hand and excluded from Dependabot.
- The Wails CLI is a build prerequisite (`wails generate module`, `wails build`),
  not a module dependency.
- Reconsider v3 once it reaches a stable release; the migration is the shell
  and the bindings, not the frontend.
- Tauri was not taken: a new language with no code to reuse, for a personal
  tool whose author already has a Wails codebase.
