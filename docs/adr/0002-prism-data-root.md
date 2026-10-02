# ADR-0002: Which Prism data root the launcher uses

**Status:** accepted, 2026-09-28; amended 2026-09-30 (the launcher creates a
chapter's instance folder itself). **Deviates from the handoff's
recommendation; read the trade-off before relying on it.**

## Context

Prism's `--dir <path>` sets a custom application root. The handoff (§3, ADR-2)
recommended a dedicated root under the app's own data directory (option B) to
isolate Kapital instances from the user's own, at the cost of a second
Microsoft sign-in inside that root.

The cost is larger than one sign-in. A Prism root holds the account, the Java
runtimes Prism downloaded, its settings, and the shared Minecraft assets and
libraries (hundreds of megabytes). A second root duplicates all of it, and a
user who fixes a Java or memory setting in their normal Prism finds it not
applied here. For a launcher whose users are the author and friends who
already run Prism, that is friction with no upside beyond name isolation.

Name isolation has a cheaper answer: instance ids are folder names, and the
launcher owns the `kapital-` prefix.

## Decision

- **Default: the user's own Prism root.** No `--dir` is passed. Instances are
  named `kapital-<chapter>` and nothing else in the manifest may name an
  instance without that prefix (`manifest_test.go` checks the bundled one).
- **A dedicated root is a setting**, `AppSettings.PrismRoot`, passed as `--dir`
  when set. Anyone who wants the isolation the handoff described turns it on
  and signs in there once.
- The launcher writes one thing into a Prism root: a chapter's instance
  folder, `<instances>/kapital-<chapter>/`, created only when it does not
  exist and never touched again (#22). Everything else in the root, the
  account, Java, settings and every other instance, stays Prism's.

## Consequences

- Zero setup for someone who already has Prism signed in: install, open, Play.
- A user who already has an instance called `kapital-luxemburg` collides; that
  is the case the setting exists for.
- `--dir` combined with `--launch` is still `[verify]` on every OS, as the
  handoff asked; milestone 3 does it against a real instance.
- Reopen if instance management (milestone 4) turns out to need to write into
  the root directly: a dedicated root would then be the safer default.

## Amendment, 2026-09-30

Milestone 4 did turn out to need it. Prism 11.1.1's `-I` always opens the New
Instance dialog, takes the folder name from that dialog's name field, and
answers a collision with a `(1)` suffix (ADR-3, Open). `--launch` needs the
exact `kapital-<chapter>` folder, so an import cannot deliver one.

The author chose to keep the player's own root as the default and let the
launcher write that one folder, over switching the default to a dedicated
root. What keeps it narrow:

- The folder is created with a plain `mkdir`, so an existing one, whoever
  made it, is refused rather than overwritten.
- `instance.cfg` is written last, so Prism's folder watcher never picks up a
  half-written instance; a failure removes the folder the launcher created,
  and only that.
- The contents are Prism's own formats (`instance.cfg`, `mmc-pack.json`) and
  the two pinned packwiz jars; no other file in the root is read or written.

## Second amendment, 2026-09-30

A player's machine is not the manifest's: 8 GB is more than an 8 GB Mac can
spare, and a preset that helps one machine may not suit another (#36). The
author chose to let the launcher change two things about an instance it
created, on the player's request from the chapter's own settings, over
sending players into Prism's instance settings:

- The heap's maximum (`OverrideMemory`, `MinMemAlloc`, `MaxMemAlloc`) and the
  JVM preset (`OverrideJavaArgs`, `JvmArgs`), and only those keys, in
  `instance.cfg`. Every other line is copied through unchanged, with its own
  line ending, and the file is replaced atomically.
- The value is held to the launcher's fixed preset list and the machine's
  memory; a player still never types an argument that reaches the game.
- A save is refused while the instance looks to be running (its game log
  changed within the last minute), so Prism and the launcher never write the
  file at once.

**Note, 2026-10-02 (#126).** The "looks to be running" test is a guess, and the
window showed it. On Windows the minute runs from the game's close, not from its
last write. Microsoft, "File Times": "When writing to a file, the last write
time is not fully updated until all handles that are used for writing are
closed." The game holds `latest.log` open for the whole run, so the stat can look
old while it runs and fresh the moment it quits. The guess stays, because it is
the last line of defence for a game started from Prism itself, and
`runningWindow` is unchanged. It is checked fresh at every write
(`SaveChapterSettings`, `SetPackSource`, the pre-launch rewrite), never read once
and trusted. The tracker is the exact answer for the launcher's own runs and is
asked first; a refusal says which of the two spoke. The guess never disables
anything in the launcher's window: the panel shows it as a hint, and a write the
guess wrongly refuses is tried again a moment later.

Optional mods, the other half of #36, wait for `kapital-packs` to mark them
and will write `packwiz.json` under the same rules.

## Third amendment, 2026-10-01

Following a start the launcher began (#44) reads two more things from the
player's side, and writes nothing. It reads the instance's
`minecraft/logs/latest.log` as the game writes it, only to match a handful of
marker lines (the window opening, the resource reload, the main menu, the
stop); a line is matched and dropped, and no line or part of one is logged,
stored or emitted. And it looks up the child processes of the Prism it
started, by pid, parent pid and name, to wait on the game's Java; it does not
read their command lines, memory or anything of Prism's account data.

## Fourth amendment, 2026-10-01

The pack sync runs headless now (#95): packwiz-installer's `-g` goes into the
pre-launch command the launcher writes, so its window never opens. An instance
made before that keeps the old command, and the author chose to have the
launcher bring it up to date over leaving every installed instance to be
reinstalled. So, before a launch, the launcher may rewrite one more key,
`PreLaunchCommand`, in an instance it created:

- Only from its own earlier template to its current one, keeping the pack URL.
  The command is parsed against the template (the same jar paths and flags, and
  a URL that passes the same check as one read back from an instance); anything
  else in that key, hand-edited or another tool's, is left alone and logged
  without its content.
- Every other line is copied through unchanged with its own line ending, and
  the file is replaced atomically, as a settings save does (second amendment).
  It is skipped while the instance looks to be running.
- It refuses nothing: a failed rewrite is logged and the launch goes on.

## Fifth amendment, 2026-10-02

Prism does not exit when it stops a start before the game: its console window
opens with the error and Prism lives until it is closed, so a failed pack sync
left the loading card at "Starting" until the start timeout (#103). While a
start waits for the game, the launcher now also reads one more file, and
writes nothing: Prism's own `logs/PrismLauncher-0.log` under the data root.

- It is read for one marker only, a line that names the `launcher.task`
  category, a `LaunchTask(` and `failed:`, which ends the start as failed. The
  line's reason is classed as the pack sync (it says the pre-launch command
  failed) or as another launch step, and nothing of the line is kept.
- No line, and no part of one, is logged, stored or emitted: the log carries
  local paths and may carry names. What is logged is the chapter and the class.
- Judged against a snapshot taken before Play, as the game log is, and read
  only until the game's own log begins.

## Sixth amendment, 2026-10-02

Prism's console is where a failed start or a crash is explained today, and the
launcher means to hide it, so a player is left with nothing to read. The author
chose to have the launcher show its own account of the run, in the loading card
and in the launcher's window (the run report), over keeping Prism's console as
the only place. That needs the one thing the third amendment ruled out: a line
of the game's log leaves the file. It does so only here, and keeps nothing.

- **When.** On the player's request, by opening the report (Details beside Play,
  `GetRunReport`), and when a run ends `crashed` or `failed` while the card is
  up, which has the report pushed to it with the state. A run the player
  stopped from the launcher is not one that went wrong and has none. Never
  while the game runs, and never on a timer.
- **What is read.** The last 16 KiB of the run's own `latest.log`, from a whole
  line, and only when the run had a fresh game log (the JVM ran): the log an
  earlier run left is never shown as this one's. The listing of the game's
  `crash-reports` folder, for the name of the newest file written since Play;
  the file is not opened. And, when the tail does not hold it, the head of the
  log up to 8 MiB, for the one line that gives the player's in-game name.
- **What is shown.** The tail, redacted, in the launcher's own window and card:
  the home path, the OS user name, the profile name, every manifest server
  address, IPv4 addresses, UUIDs, the values after `--accessToken`, `--uuid`,
  `--username` and similar launch arguments, and the player's in-game name as
  the log gives it (`Setting user:` or the `--username` argument), as a whole
  word wherever it appears. The crash report is a file name, never a path.
- **What is kept.** Nothing. The report is built for the call, returned, and
  held by the view that shows it alone, and gone with it. The tracker holds
  where the run's log is and when each phase was reached, which is not the log.
  No line, and no part of one, is logged, written to disk or sent anywhere: what
  is logged is the chapter, the phase, the number of lines and whether a crash
  report was named. The card's copy is the clipboard, as ever, and holds the
  launcher's own log (S7.3), not this.

The report is refused when the redactor cannot be built, rather than made
without one, and carries `consoleAvailable`, which is true when the console the
launcher hid (Windows, ADR-12 amendment) can be shown back on request.

## Seventh amendment, 2026-10-02

An instance installed from a dev pack (#41) kept `http://localhost:8080/pack.toml`
in its pre-launch command for good: the fourth amendment never rewrites a URL,
so clearing `packOverrides` changed nothing and the only way back to the
published pack was deleting the instance in Prism and installing again. The
author chose to let the player switch one instance between the two packs over
making them reinstall. So the launcher may rewrite `PreLaunchCommand` once more,
on the player's request:

- **When.** From the chapter's own settings (the pen in the hero, "Pack source",
  `SetPackSource`), for an installed chapter, in either direction. Never on its
  own, and never before a launch.
- **Between two values it knows.** The manifest's `pack.toml` for the chapter
  (`published`) and the loopback address from `packOverrides` (`dev`), the
  caller naming a chapter and one of those two words and never a URL. The
  override is held to `CheckLocalPackURL` again at the call, and the new URL to
  the command-line character rule.
- **The template check of the fourth amendment applies.** The key must be
  exactly the launcher's current template or its earlier one for the URL it ends
  in (`SwitchPackSource`, `packswitch.go`). Anything else, hand-edited or another
  tool's, is refused with an error that says the launcher did not write it, not
  rewritten, and nothing of the command is in the error or the log. A command
  that already names the chosen pack is not written.
- **How it is written.** The one key, in the current template, atomically, every
  other line and its line ending kept, as a settings save does (second
  amendment). It is refused while the game is active, and while the instance
  looks to be running.

What packwiz-installer does on the next Play (read in its `ManifestFile.kt` and
`UpdateManager.kt`): its `packwiz.json` in the game folder holds file hashes and
no URL, so it syncs by the new pack's index. It removes the files only the other
pack had, adds the ones only the new pack has and updates the rest. Saves,
options and the optional-mod choices are left alone. Nothing else in the instance refers to the pack URL, so one instance and
the one key are enough.
