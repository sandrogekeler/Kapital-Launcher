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

## Eighth amendment, 2026-10-04

The run report (sixth amendment) shows the end of the latest run's log and
nothing before it. An earlier run's log, a crash from yesterday and a run started
from Prism directly are out of reach, and a player asked for "the log from when
it crashed" has to find the instance folder and unpack a `.log.gz`. The author
chose a Logs page per chapter (issue 155), opened from a third tool in the hero,
over sending the player to the folder. The launcher now reads more of the game
folder, on request, read only:

- **What is listed.** The instance's own `logs/latest.log`, `logs/*.log.gz` and
  `crash-reports/*.txt`, newest first, at most 50 of each kind, with each file's
  name, modified time and size. Listing opens no file.
- **The crash mark.** A log is marked as crashed when a crash report's modified
  time falls in its run: after the next older log was archived (the game
  archives a run's log at the next start, so that is when this run began) and by
  the log's own last write plus two minutes. The oldest log, with no older one,
  is taken to have begun a day before its end. It is a rule over file times and
  is best effort.
- **What is read.** One file at a time, named by kind (`log` or `crash`) and by a
  base name the listing would produce. Go resolves it inside the instance's game
  folder through an `os.Root`, so a link out of the folder is refused, and
  refuses a name with a separator, `..`, a stream marker, or the wrong shape. A
  file is read a chunk at a time: the last 256 KiB from a whole line, or the
  256 KiB before an offset the caller was given ("Load earlier"). A dated log is
  unpacked while it is read and refused past 256 MiB unpacked.
- **What is shown.** The chunk, redacted as the run report's is (sixth
  amendment): home path, OS user, profile name, server addresses, IPv4
  addresses, UUIDs, launch-argument values and the player's in-game name, which
  is learned from the chunk, else the file's head, else `latest.log`'s head.
  Copy puts on the clipboard what is shown, through Wails' runtime.
- **What is kept.** Nothing, as before: the text is returned for the call and
  held by the page alone, which drops it on closing. What is logged is the
  chapter, the kind and the number of lines.
- **The live log.** The page can also show `latest.log` as the game writes it. Go
  follows that one file, looking at it every 500 ms through the same `os.Root`
  and name rules, and emits what was appended as `log:live` events: redacted
  lines, at most 64 KiB an event, a line still being written held back until
  its newline. A file that shrank or was replaced, a new run starting, emits a
  reset with the new file's end. One chapter's log is followed at a time, only
  while the page asks (`WatchLiveLog`, `StopLiveLog`) and until the app quits.
  The follower keeps an offset and the first 128 bytes of the file, nothing
  else; the frontend holds the last 5000 lines and drops them with the page.

## Ninth amendment, 2026-10-04

Some mods cost more than a player's machine can give (Distant Horizons above
all) or are a matter of taste (Colorwheel, Create Better FPS), and there was no
way to turn one off that lasted (issue 156). packwiz-installer fetches again any
non-optional file whose jar is missing, even when the pack has not changed
(v0.5.14, `UpdateManager.kt:137-152`, `DownloadTask.kt:231-263`), so a mod
renamed to `.jar.disabled`, Prism's own convention and the one every loader
skips (NeoForge and Forge's `ModsFolderLocator`, Fabric's
`DirectoryModCandidateFinder`: only `*.jar`), came back at the next Play. Its own
optional-file mechanism has no flag for a headless run and enables every optional
file whenever a pack update adds one (`CLIHandler.kt:48-54`). The author chose to
have the launcher wrap the sync over editing the packs or running the installer
with its window.

- **The command.** Prism's pre-launch command becomes
  `"<copy>" --prelaunch-sync <pack URL>`, the launcher's own executable. The URL is
  still last, which is what the pack source switch and the URL read back rely on.
  Prism splits the command without a shell: `substituteVariables`, then
  `QProcess::splitCommand`, then `start(program, args)` (`PreLaunchCommand.cpp`),
  so the quotes keep a path with spaces together and a `$` would be read as a
  variable, which is why a copy path with one is refused. The environment is
  Prism's own (`INST_MC_DIR`, `INST_JAVA`, `INST_ID`), the working directory the
  game folder. A GUI-subsystem program started this way gets Qt's piped stdio:
  tested with a Go program built `-H=windowsgui`, whose output, exit code and an
  argument with a space all arrived, and Wails creates no window before
  `wails.Run`, so `main()` looks at `os.Args` first and nothing of the app starts.
  Prism has no timeout on the command and cancels it with a hard kill; its output
  is read in the system code page, so the sync prints ASCII lines prefixed
  `kapital-sync:` and nothing else of its own.
- **The copy.** The command names a copy of the launcher kept in its data folder,
  not the running executable: `<data dir>/sync/kapital-launcher(.exe)`, beside a
  version file. Its path never moves, so a Play from Prism directly keeps
  working after the launcher is moved or updated, and a macOS app run from a
  translocated path cannot break it. It is made before the pre-launch command is
  written, rewritten or switched, and when the launcher starts if one exists;
  refreshed when the running version differs from the version file, when the copy
  is not the size it was written at, or when it is gone; written to a temporary
  file, then renamed over the copy, then the version file, so a Prism that runs it
  in the middle sees one whole copy. On Windows a copy a sync is running from
  moves aside to `.old` first (a running program can be renamed, not replaced). A
  dev build (`-dev` in `Version`: `wails dev`, a plain `go build`) keeps its copy
  in `sync-dev/` and also refreshes when its own file changes, since every rebuild
  shares the version; it never overwrites a release copy, and the command follows
  whichever build last played, as the rewrite before each Play names the copy of
  the build that runs it. Where no copy can be made (a data folder whose path
  Prism would misread) the instance keeps the packwiz command, and only the mod
  switches wait.
- **The run.** (1) A journal, `kapital-disabled.json` in the game folder, lists
  the jars about to be restored, written before anything is renamed. (2) Each of
  the player's disabled mods is renamed from `x.jar.disabled` back to `x.jar`, so
  the installer finds nothing missing and downloads nothing; if both exist, as
  after a pack update downloaded the mod again, the stale `.disabled` is removed
  and the new jar kept. (3) packwiz-installer runs with exactly the packwiz
  template's arguments as an array (`packwizSyncArgs`, kept equal to the template
  by a test), in the game folder, with its output on the sync's, and no window of
  its own. (4) Whatever its exit code, every disabled mod is renamed to
  `.jar.disabled` again, a jar the installer downloaded that is on the list
  included, and so is a jar of a manifest toggle whose name a list entry matches
  (the pack replaced `DistantHorizons-3.3.3` with `3.4.0`; the toggle's prefix says
  it is the same mod). (5) The journal is removed, and the sync exits with the
  installer's code. An instance that is not a chapter of the launcher, or whose
  game folder is not the instance's own, is synced as it always was.
- **What it touches.** Only regular files directly inside `<game folder>/mods`,
  through an `os.Root`, by a name the strict jar shape accepts, plus the journal.
  The list is the launcher's own, per chapter, in its settings
  (`disabledMods`, jar base names), written only by `SetModsDisabled`, which checks
  each name against the folder, saves the list, and renames at once so the folder
  matches before the next Play. It is refused while the game runs and under a
  developer preview, as the other writes are. The manifest's quick toggles
  (`pack.toggles`, a name and a jar-name prefix) are matched against file names and
  never reach a path or a command.
- **Migration.** `RewritePreLaunchCommand` brings the first and the packwiz
  template, and a sync command that names another copy of the launcher, to the sync
  command before a Play, keeping the URL; `SwitchPackSource` accepts all of them.
  A command that is not exactly one of the launcher's is left alone, as before.

What it costs, and what stays open:

- **A killed run.** Cancelling in Prism kills the sync, not the Java it started,
  which goes on downloading, and leaves the mods as jars with the journal.
  The next sync treats a jar as restored already and puts it away; the settings
  page does the same when it opens with the game closed. Until one of them runs the
  mods load if the game is started some other way.
- **A disabled mod that is not a toggle and is updated** comes back enabled: its
  list entry names a jar that is gone. Only a manifest toggle follows a version
  bump. The list entry is shown skipped in the log, and the next save from the
  page drops it.
- **The first Play** has no mods to restore: a mod on the list that the installer
  downloads is put away after it, so it is downloaded once.
- **The copy can go missing** (the data folder cleared while an instance still
  names it): Prism then fails the pre-launch step; the next Play from the launcher
  makes it again.
- **Not verified on a real run:** Windows security software and SmartScreen on a
  copied unsigned program run from the data folder; that the sync's Java child
  opens no console window (`CREATE_NO_WINDOW` is set, as Qt does); and on macOS
  that the Wails binary runs outside its bundle, which a Go program linked against
  AppKit and run before any window should, with its ad-hoc signature intact in a
  byte copy.

## Tenth amendment, 2026-10-04

The second amendment held the player to the launcher's preset list: "a player
still never types an argument that reaches the game". The author decided on
issue 191 to let the player add Java arguments of their own after the preset's,
from the chapter's Game card, as Prism's own instance window already allows
anyone with the instance open.

- **Where they live.** In `JvmArgs` itself, after the preset's arguments and
  separated by spaces, with `OverrideJavaArgs` on whenever there are any. Nothing
  is kept in the launcher's settings: `ReadChapterSettings` reads a value that
  starts with a preset's arguments as that preset and the rest, and anything
  else as no preset and all of it the player's, so arguments written in Prism's
  window (issue 190 puts it a click away) are shown and kept.
- **What is allowed.** `ValidateJVMArgs`, on every save: each is one argument
  that starts with a dash, with no space, quote, backslash or control character,
  because Prism splits `JvmArgs` on spaces and reads quotes and backslashes
  itself, so one entry can never become two; at most 200 characters, 32
  arguments, no repeat. Refused whatever the player wants: memory (`-Xmx`,
  `-Xms`), which the slider owns; an agent (`-javaagent`, `-agentpath`,
  `-agentlib`), which loads code into the game; the two options that run a
  command when the JVM fails (`-XX:OnError`, `-XX:OnOutOfMemoryError`); and the
  two that read more options from a file (`-XX:VMOptionsFile`, `-XX:Flags`). An
  argument Prism's window wrote that breaks these is shown, and the save says
  which one to remove.
- **What does not change.** A manifest still names a preset and never an
  argument (S2.1); the launcher writes the same five keys, atomically, refused
  while the game is active; nothing of `JvmArgs` is logged.

## Eleventh amendment, 2026-10-05

The hero can show a chapter's title-screen panorama as a slowly turning cube
(issue 195). Each pack ships a resources pack of its own, and the six faces of
the game's menu background are in it at the vanilla path. The launcher reads
them, which is one more read inside a chapter's own instance.

- **What is listed.** The entries of `<game folder>/resourcepacks/` of an
  installed chapter, by name only (`minecraft`, or `.minecraft` in older
  instances). A name that contains "resource", in any case, and is a folder or a
  `.zip` is a candidate; the first in sorted order that holds all six faces is
  used. Nothing else of the folder is opened.
- **What is read.** Six files at one fixed path inside that one entry,
  `assets/minecraft/textures/gui/title/background/panorama_0.png` to
  `panorama_5.png`: through an `os.Root` on the folder, so a link cannot lead
  out of it. For a zip only the central directory is read, then the six entries.
  Each is read under a 4 MiB cap, decoded as a PNG (the dimensions first, at
  most 4096 pixels square), and must be square and of one size with the other
  five. Anything else refuses that pack.
- **What is kept.** Copies of the six PNGs in the launcher's own data dir,
  `panorama/<chapter id>/`, with a key made from the pack's name and the file
  sizes and times (the zip's, for a zip), so a changed pack is read again and an
  unchanged one is not. The cache of a chapter with no panorama is removed. The
  copies are served to the launcher's own page at `/panorama/<chapter>/panorama_<n>.png`
  (GET and HEAD, that name shape only, through an `os.Root`), so the page's CSP
  stays closed. Nothing of the pack is logged but the chapter and the face size.
- **When.** On the page's request (`GetPanoramas`), made only while the
  panorama is the chosen picture: when it is chosen, on window focus, and after an
  install or a game run ends. Never on a timer.
- **What does not change.** Nothing is written into the instance, no other file
  of a resources pack is opened, and no account data is touched.
