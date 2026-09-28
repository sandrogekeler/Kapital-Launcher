---
paths:
  - "frontend/src/stores/**"
  - "frontend/src/lib/ipc.ts"
  - "app.go"
---

# IPC conventions

**Go owns all side effects** and **IPC goes through the generated bindings
only**, imported from `wailsjs/go/`, never raw `window.go`. Those two hold before
any file is opened and live in `agent_docs/CLAUDE.md`. The rest is here.

## Shape

- Bound methods live on `App` in `app.go`, `PascalCase`, and return `(T, error)`.
  `app_test.go`'s `TestBoundMethodsReturnAnError` reads the file and fails on one
  that does not.
- Re-run `wails generate module` after adding or changing one; `frontend/wailsjs/`
  is generated and never edited.
- A bound method that takes an id from the frontend resolves it through the
  validated manifest (`a.chapter(id)`) and uses the manifest's values, never
  the caller's. `LaunchChapter` and `OpenChapterWiki` are the pattern.

## Where errors are handled

In the store that owns the data. Each store holds its own `error`; a write
action reverts and rethrows so a caller can react; a read degrades to a
sensible value (the bundled manifest, `null` for the engine).

`lib/ipc.ts` separates the two things a rejection means:

- **No bridge** (`hasWailsBridge()` is false): the browser-only `frontend-dev`
  preset in `.claude/launch.json`. Nothing was ever going to persist, so an
  optimistic write stands and a read falls back. `readOr` is the read helper;
  it awaits inside so the bindings' *synchronous* no-bridge throw becomes a
  rejection one handler covers.
- **Bridge present**: a rejection is a real failure. Revert, record, rethrow.

A bare `catch {}` that swallows a rejection is the thing to refuse in review.

## Events

None yet. When one arrives (pack sync progress, server status), it is emitted
from Go with `runtime.EventsEmit`, listened to with `EventsOn` from
`wailsjs/runtime`, and cleaned up on unmount. Do not poll for what an event
can deliver.
