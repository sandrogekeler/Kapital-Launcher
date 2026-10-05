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

## Amendment, 2026-10-02: Prism's console, hidden, kept and shown on request

When a start fails, Prism opens its console window with the error
(`ShowConsoleOnError`, its default). The author wants the launcher's own view
of the failure (the run report, ADR-2 sixth amendment) in its place, with a way
to open Prism's console from it. The console is not thrown away: it holds what
exists nowhere else, the pre-launch output (confirmed in Prism 11.1.1's
source: the launch log is never written to a file).

- **Hidden with the same hold, kept past the run.** `HoldPrismConsole`
  (`gamewindow_console_windows.go`) is the dialogs' hold (`startHolder`) on the
  launcher's own Prism's pid, started with it and on the same condition, the
  splash being on, and matching a top-level window whose title begins `Console
  window for` (seen on a real PC, Prism 11.1.1: `Console window for <instance>
  - Prism Launcher 11.1.1`). Unlike the dialogs' hold it is not released at the
  handover or when the run ends: Prism opens the console at about the moment
  the run ends (`LaunchController::onFailed` shows it, then logs the failure),
  so a hook that stopped with the run could miss it. It lives on a per-chapter
  record in the tracker, and its windows are never shown back automatically.
- **Shown on request.** The run report says `consoleAvailable` when the hold
  has a window left on a Prism that is still running, and the card and the
  launcher's panel offer "Show Prism's console": the bound `ShowPrismConsole`
  and the card's fourth action, `showConsole`. Showing ends the hook first, or
  it would hide the window again as it is shown, then shows the held windows
  and gives them the foreground.
- **The card's button follows the console, not the run's end (#208).** Since
  Prism's own log ends a failed run (ADR-2 fifth amendment) it does so about
  0.4 s before Prism opens its console (seen 2026-10-05: failed at 23:02:21.040,
  console held at 23:02:21.45), so the report the card builds when the run ends
  said `consoleAvailable` false and was never built again. The holder is given
  an `onHeld` call, made once when its first window is held, which the tracker
  turns into `TrackRequest.OnConsoleHeld` and the app into `SplashCard.ConsoleHeld`.
  For a card that is up, shows an end and has a report, that sets the report's
  `consoleAvailable` and pushes the card again; a card the player left, a run the
  player stopped (it has no report) and a start still in progress are left alone.
  A console that comes first is read as before when the report is built. No
  polling, and nothing changes in the page, which already draws the button from
  the report.
- **A second failure signal.** A console hidden while the run is still in
  `starting` with no game log of its own is a launch step having failed, and
  ends the run `failed`, reason `launch`, after a second for Prism's own log to
  say which (ADR-2 fifth amendment, ADR-11: the line, when it is read, still
  gives the reason). One that appears once the game's log is fresh is the
  player's own (`ShowConsole=true` in their Prism) and changes nothing.
- **Prism's lifetime is the launcher's, until the next Play, Stop or quit.** A
  Prism that has a hidden or shown console is alive only because of it, and is
  closed when the player presses Stop, when the next Play of that chapter begins
  (before the new Prism starts, or it would hand its launch to the old one) and
  when the launcher quits (bounded, so quitting never waits long). Closing is the
  Stop routine's: `WM_CLOSE`, then ended after five seconds if it is still
  there. The console's windows are hidden, which the routine's "visible windows
  only" rule would skip, so the hold's own handles get the `WM_CLOSE` as well;
  Prism exits 0 once none is left. A Prism with no console left, say one with a
  game running, is left alone. The player closing the shown console ends Prism
  too, which is Prism's own behaviour.
- **What is read.** The title's first 32 characters, through `GetWindowTextW`,
  to match the prefix: never kept, never logged. Only the launcher's own
  Prism's windows, by pid. A Prism in another language matches nothing and shows
  its console as it always did, the safe failure.
- **Windows only.** macOS has no window holder, so Prism's console appears as
  Prism makes it and the view has no button for it.

## Amendment, 2026-10-05: the Prism a run leaves behind is on a record with no console too

The record above existed only when the run held the console, which needs the
splash. With the splash off a failed start leaves the launcher's Prism alive on
a console of its own, nothing was hidden, and the next Play had no record to
close it by: its `--launch` was handed to that Prism (issue 211, seen on
2026-10-05 with the pack server down).

- **The record is made for every run.** `holdPrismConsole` records the Prism
  the launcher started (its pid and exit channel, from `TrackRequest.Prism`)
  whether or not a console could be held; the holder is nil when none was (splash
  off, no hold on the platform, the hook failed to start). A nil holder offers
  nothing to show (`consoleAvailable`, `ShowConsole`) and is not a failure signal
  (`consoleFailed`).
- **Closed when it is left behind.** With a holder, as before: while a console
  window is left. With none, once the run that started it has ended (the record's
  `over`, set when the run's goroutine finishes): a Prism whose run is still going
  has a game the player is in, and quitting the launcher leaves it. The close is
  the same routine, by the pid the launcher started and never by name (S3.9), and
  a Prism that exits by itself releases the record as before.
