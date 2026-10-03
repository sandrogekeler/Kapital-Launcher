# How a pack reaches a player

A plain account of what happens between editing a modpack and a friend
pressing Play: what a pack is, where it lives, how it is published, what
Install and Play do, how an update gets out, and what the dev pack is for.
Each part names the real system doing the work, so the technical record in
`docs/adr/` (mainly ADR-2, ADR-3, ADR-4 and ADR-11) reads as the same story.

## The cast

| Who | What it is | Its job here |
|---|---|---|
| **packwiz** | A small command-line tool for Minecraft modpacks | Writes the pack's recipe and keeps its checksums right |
| **`kapital-packs`** | A private git repository, one folder per chapter | The source of truth for every pack |
| **The pack host** | A Cloudflare Worker at `kapital-packs.alessandrogekeler.workers.dev` | The published copy of each chapter folder, read-only, on the web |
| **Modrinth and CurseForge** | The two public mod sites | Where the mods themselves are downloaded from; the pack never re-hosts a mod |
| **Prism Launcher** | The engine: Microsoft sign-in, Java, the mod loader, the game | Owns the instances and starts the game |
| **packwiz-installer** | packwiz's companion, two small Java programs placed in an instance | Runs inside Prism before every start and brings the instance in line with the pack |
| **Kapital Launcher** | This app | Finds Prism, writes the instance, presses Prism's buttons, shows what is going on |
| **The manifest** | `data/launcher.json`, built into the app | The chapter list: names, art, server address, and for each chapter the address of its published pack and the version shown under the title |

## A pack is a recipe, not a bundle

A packwiz pack is a folder of small text files, not a zip of mods:

- `pack.toml` names the pack, its version, the Minecraft version and the mod
  loader, and points at the index.
- `index.toml` lists every file in the pack with a checksum (SHA-256).
- One small `.pw.toml` file per mod says where that mod's jar comes from:
  a download on Modrinth or CurseForge, with its own checksum.
- The files the pack carries itself: configs, KubeJS scripts, the menu art,
  the pack's own resource pack. These are stored in the folder as they are.

So the repository holds the recipe and the pack's own files, about 107 MB for
Frangfurd, and none of the mod jars. Anyone with the recipe can rebuild the
same pack, file for file, because every entry has a checksum. ADR-3 chose this
over an admin panel: adding a mod is `packwiz modrinth add <name>`, removing
one is `packwiz remove`, and `packwiz refresh` rewrites the index with fresh
checksums. Every change is a commit with a diff.

## Publishing is a push

The pack host is built automatically from the repository (Cloudflare Workers
Builds). Pushing a change to `kapital-packs` republishes the chapter folders
at the Workers address shortly after. The two files that matter for
freshness, `pack.toml` and `index.toml`, are served with caching off, so a
player's next start sees the change at once.

Nothing is uploaded by hand, and there is no "release" step for a pack
beyond the push. A git tag such as `frangfurd-v1.0.0` marks a version in the
history for people; packwiz-installer does not look at tags.

Everything inside a chapter folder becomes public on the host. The private
things, the repository itself and the wiki, stay private; what is in a
chapter folder (configs, scripts, art, the resource pack) is not.

**Two version numbers.** `pack.toml` carries the pack's version, which is
what packwiz and the launcher's update line read. The manifest carries
`pack.version` for the same chapter, which is what the launcher prints under
the title ("Pack 1.0.0"). Today both are edited by hand and should be bumped
together. ADR-4 plans to have the wiki publish the manifest later, which will
make that one edit.

## Install writes an instance and downloads nothing of the pack

Pressing Install in the launcher creates the chapter's instance inside Prism,
in Prism's instances folder, under a fixed name such as `kapital-frangfurd`:

- `instance.cfg`: Prism's settings for the instance, with the memory and the
  Java preset from the manifest, and one line Prism runs before every start,
  the **pre-launch command**. That command runs packwiz-installer with the
  pack's address. It is the whole link between the instance and the pack.
- `mmc-pack.json`: the Minecraft version and the mod loader, read from the
  hosted `pack.toml` and checked against the manifest. Prism downloads the
  loader and a matching Java on the first start.
- The two packwiz-installer programs (`packwiz-installer-bootstrap.jar` and
  `packwiz-installer.jar`), fetched once from their GitHub releases and
  checked against pinned checksums.

That is all. No mod is downloaded at Install. The state line says so: "The
first Play downloads the pack". The launcher writes this folder only when it
does not exist, and after that touches `instance.cfg` only for the settings
it owns (ADR-2 and its amendments say exactly which keys).

## Play is the sync

Play asks Prism to launch the instance. Prism runs the pre-launch command
first, and only if it succeeds starts the game. packwiz-installer does the
following, every single start:

1. Downloads `pack.toml` from the pack's address, then `index.toml`.
2. Compares the index with its own record from last time, `packwiz.json` in
   the instance's game folder, which lists every file it placed and its
   checksum.
3. Downloads what is new or changed: mods from Modrinth or CurseForge, the
   pack's own files from the host. Every download is checked against the
   checksum in the recipe and refused on a mismatch.
4. Removes files the pack no longer lists, and leaves everything the pack
   never listed alone: saves, options, screenshots, the player's own
   resource packs.
5. Writes the new record and exits. Prism then starts Java.

So "installing the pack", "syncing" and "updating" are the same step seen on
different days. The first Play is a full download and takes a while. A later
Play with nothing changed is quick, since nothing is downloaded. A Play after a
push downloads only the difference.

When the step fails, because the host or a mod site could not be reached or
a file's checksum did not match, Prism stops before the game and the launcher
reports "The pack could not be synced". Nothing half-done is kept as the new
record, so the next Play tries the same difference again.

## Updates are pulled, never pushed

There is no mechanism that pushes an update to a player's machine. The author
pushes to the repository, the host republishes, and each player's next Play
picks the difference up. A player who does not press Play does not update.

The launcher does not offer an update. Every Play syncs the pack before the
game, so Play always reads Play, and the bar's pack line (for a chapter
without a server) reads "● Updated" over "Version X", where X is the
published pack version, the one the next Play syncs to. When the app starts
and whenever it regains focus, it downloads the published `pack.toml` and
compares its checksum with the one packwiz-installer recorded at the last
sync; the Version row in the chapter's settings uses that check to say
"older than X" when the installed pack is behind. There is nothing to click.

## The dev pack

`packwiz serve` is packwiz's own small web server. Run inside a chapter's
folder of the repository, it serves that working copy on
`http://localhost:8080/`, exactly the way the host serves the published copy.
It lets a change be played before it is pushed.

The launcher's Developer settings have a field per chapter for that address
(loopback addresses only). With it filled, Install writes the instance's
pre-launch command against `localhost:8080` instead of the host, and the
state line reads "Dev pack · Syncs from localhost:8080". The instance then
syncs from the working copy on every Play, including the files in it that are
not committed yet.

The address lives in the instance, not in the setting: clearing the field
changes nothing already installed. The chapter's own settings (the pen in the
hero) have a "Pack source" section that switches an installed chapter between
the published pack and the dev pack. The next Play re-syncs from the chosen
one; because packwiz-installer works from file checksums and not from the
address, it removes what only the other pack had, adds what the new one has,
and leaves saves and options alone. The switch is refused while the game runs
(ADR-2, seventh amendment).

A working day with a dev pack, then:

1. Edit the pack in the repository with packwiz, and run `packwiz refresh`
   so the index's checksums match the files.
2. `packwiz serve` in the chapter folder; switch the chapter to the dev pack
   if it is not already; Play.
3. Happy? Commit and push. The host republishes.
4. Switch the chapter back to the published pack, or leave it on the dev
   pack while the next change is in progress. Players never see any of this;
   only an instance installed with the field filled in points at localhost.

A sync from the dev pack fails the same way a published one does when
`packwiz serve` is not running: "The pack could not be synced".

## Three things that update separately

"Update" means three different things in this project, and they do not move
together:

| What | How it updates | Who decides when |
|---|---|---|
| **A pack** | Pulled by packwiz-installer at the next Play, as above | The author, by pushing; each player, by pressing Play |
| **Prism Launcher** (the copy the launcher installed) | The launcher reads Prism's latest release at start and offers the update on a click, verified like the first install (ADR-11) | The player, on the click; a player's own Prism is theirs |
| **Kapital Launcher itself** | By hand: download the new build from the repository's releases (ADR-6; an in-app check is a later step) | The player |

Java and the mod loader are Prism's: it downloads what the instance names,
and the pack's `pack.toml` names them.

## Where things end up on a player's machine

| Thing | Where |
|---|---|
| The launcher's settings and log | The app data folder, `KapitalLauncher` under the OS's application data |
| The managed Prism program | `prism/app-<version>/` under that folder, replaced whole on an update |
| Prism's data: accounts, Java, instances | `prism/root/` under that folder, never touched by an update; a player's own Prism keeps its own data folder |
| The chapter's instance | `instances/kapital-<chapter>/` inside Prism's data |
| The synced pack: mods, configs, the pack's files | `minecraft/` inside the instance, with `packwiz.json` as the sync record |
| Saves, options, screenshots | The same `minecraft/` folder, and the sync never touches them |

## Where the details are

- ADR-3: the pack format and hosting, and why a repository rather than a panel.
- ADR-2 and its amendments: exactly what the launcher writes in Prism's
  folders and when.
- ADR-4: the manifest, and the plan to have the wiki publish it.
- ADR-11: how the launcher gets and updates Prism for a player without one.
- `docs/HANDOVER.md`, "The packs": the repository, the host and the tools
  around them as they stand.
