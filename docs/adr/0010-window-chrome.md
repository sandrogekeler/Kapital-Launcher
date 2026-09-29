# ADR-0010: The app draws its own window header

**Status:** accepted, 2026-09-29.

## Context

The OS title bar is a grey strip that matches nothing else in the window and
has no room for the app's own controls; settings (#5) needed an entry point
that is not tied to one chapter. The author asked for a header like a
browser's tab strip: the brand at the left, the gear at the right, and no
system bar.

Electron can hide the title bar and keep Windows' real caption buttons
(Window Controls Overlay). Wails v2 cannot: a frameless window on Windows
loses the caption buttons with the bar. macOS can: a hidden title bar keeps
the traffic lights.

## Decision

- **A full-width header bar, `layout.titlebar` tall**
  (`components/shell/HeaderBar.tsx`). Left: the Kapital Launcher logo (the
  author's artwork, kept full size in `../Art`), which replaces the Kapitel
  Kapital wordmark the sidebar used to carry. The wordmark file stays in
  `frontend/src/assets/brand/`: the download site (ADR-9) still shows it.
  Right: the settings gear. The bar's empty area drags the window through
  Wails' `--wails-draggable` property, and a double-click maximises; its
  controls opt out of both.
- **Windows (and Linux): frameless, with our own buttons.** Minimise,
  maximise or restore, and close, 46px wide at the bar's height, close red
  on hover, glyphs from `lib/icons.ts`. They call the Wails runtime.
- **macOS: a hidden title bar, native traffic lights.** No window buttons are
  drawn; the brand starts after the lights. Full-screen, zoom and the
  window menu stay the system's.

## Consequences

- Windows 11's Snap Layouts flyout on hovering maximise is gone; Win+Z,
  Win+arrow and dragging to an edge still snap, since the drag is a native
  caption drag.
- Resizing from the edges is Wails' frameless resize handling on Windows.
- `[verify]` on macOS: that `TitleBarHidden` places the traffic lights inside
  the 40px bar and that `pl-20` clears them.
- Reopen when moving to Wails v3 (ADR-1) if it offers the real caption
  buttons without the bar.
