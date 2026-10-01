# ADR-0012: Hiding the game window until the handover

**Status:** accepted, 2026-10-01. Built as a prototype (#45) behind the
developer setting `holdGameWindow`, off by default until the loading splash
(#43) covers the time the window is hidden.

## Context

The loading splash (#43) is only a splash if the game's own window stays out
of sight until the splash hands over. #45 offered three routes: the launcher
hides the window with OS calls, a pack-side mod keeps it hidden, or the first
with the second as a fallback. The author chose OS calls on Windows first.

Measured on the author's PC on 2026-10-01: Windows 11, Frangfurd (NeoForge
21.1.252, Minecraft 1.21.1), the managed Prism 11.1.1. A probe listed every
top-level window of Prism and of each Java process every 100 ms, and the
launcher logged its own hide times.

- The game's window is class `GLFW30`, created hidden and shown about 0.7 s
  later at the early window's size, then resized to the player's fullscreen.
- **With `earlyWindowControl = false`** (Frangfurd 1.0.0, #46) a hide from the
  launcher landed after **19 s**: `ShowWindow` on another process's window is
  carried out by that window's thread, and the game's thread handles no
  messages while it loads mods. The window was in plain view until the resource
  reload began.
- **With `earlyWindowControl = true`** FML's early window keeps handling
  messages. The hide landed **0 to 31 ms** after the window was shown, on every
  one of five starts, and the probe never saw the window. Loading from the
  window to the reload's end took 42 s hidden and 42 s shown.
- With the early window on, the game came up fullscreen but drew at 854x480 in
  the bottom-left corner on three of four starts (the glitch #46 removed the
  early window for). Shrinking the fullscreen window by one pixel and restoring
  it fixed it on every start it was tried.
- The game window took the foreground at the handover on some starts and not
  on others: Windows hands the foreground over only while the launcher holds
  it.

## Decision

- **OS calls, with the pack's early window on.** Once the game tracker (#44)
  has the game's Java, the launcher hooks that process's show events
  (`SetWinEventHook`, out of context) and hides a top-level `GLFW30` window the
  moment it is shown, again on any later show. Frangfurd turns its early window
  back on (kapital-packs#3) so the hide lands at once.
- **The handover is the resource reload beginning**, not its end, so the
  pack's own loading screen (Drippy) is seen. At the handover a fullscreen
  window is made one pixel shorter and restored, then shown and given the
  foreground. Every call there is queued to the game's thread
  (`SWP_ASYNCWINDOWPOS`, `ShowWindowAsync`), so a busy game never stalls the
  launcher.
- **Any end before the handover shows the window**: a crash, a failed start or
  the launcher closing never leaves a hidden window behind.
- **Only the game's `GLFW30` window is touched**: its class, owner and
  rectangle are read, never its title or input, and no process is started.
- **Windows only.** macOS has no equivalent without the Accessibility
  permission; its answer goes to #30.

## Costs

- It depends on the pack: a pack with the early window off is hidden late. Each
  chapter's loader needs the same check: Lichdenstein is Fabric, Luxemburg
  Forge 1.19.2, neither measured.
- A start from Prism directly gets the early window and may show the corner
  glitch, since no launcher nudges it.
- Prism's own progress dialogs and packwiz-installer's window are not the
  game's. #95 hides Prism's "Please wait" dialogs with this same hold (by their
  title, the one title the launcher reads, and only for the Prism it started)
  and runs packwiz-installer headless so it has no window; #50 decides the
  rest.
- The foreground is Windows' to give. When the launcher is not in front at the
  handover, the game may open behind other windows.
- Until #43 exists the setting is a developer one: with it on and no splash, a
  player sees nothing for about a minute.
