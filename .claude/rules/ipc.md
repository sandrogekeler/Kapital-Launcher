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
  the caller's. `LaunchChapter`, `InstallChapter` and `OpenChapterWiki` are the pattern.

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

Three: `server:status`, a `models.ServerStatus` emitted by
`StatusService.Run`'s ticker and by `GetServerStatus`; `prism:install`, a
`models.PrismInstallProgress` per step while `InstallPrism` runs, heard by the
engine store (`EVENT_PRISM_INSTALL`), the install's outcome still coming from
the promise, so a missed event cannot leave it hanging; and `game:state`, a
`models.GameState` per phase change of a launched game, emitted by
`GameTracker` (`EventGameState`), heard by the game store
(`EVENT_GAME_STATE`), with `GetGameStates` for the state now; it carries the
splash flag and the start's estimate (#43), and `LeaveSplash` is the one call
that gives the launcher's window back to the player. The
pattern for the next one:

- The name is a Go constant (`services.EventServerStatus`) and a TS constant
  (`EVENT_SERVER_STATUS`), spelled the same.
- The payload names what it is about (`chapterId`), and the listener files by
  it. A payload without one is dropped.
- One `EventsOn` per event, in the store that owns the data, subscribed from
  App for the app's lifetime and returning the `EventsOff` for a StrictMode
  double mount. With no bridge, `listen` subscribes to nothing.
- Do not poll for what an event can deliver. `check` exists for "now", not
  for a timer.
