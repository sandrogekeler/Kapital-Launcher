# Kapital Launcher: Security Checklist

How each security-relevant part of the app must be, how to verify it, and what
to try to break. The target, not a snapshot: an item that turns out wrong is
corrected here with the reason. Only `Open backlog` should churn.

**Who the attacker is:** anyone other than the person running the launcher.
Two reaches matter: **network** (a manifest, a pack index, a download, a server
ping response) and **bridge** (a bound method on `App`, callable by anything
that runs in the WebView).

Bound methods on 2026-10-04: **41** (`grep -c '^func (a \*App) [A-Z]' app.go app_*.go`,
summed). A different count is new surface to classify: add the method to this table.

| Method | Takes from the bridge | Reaches | Item |
|---|---|---|---|
| `GetAppVersion`, `GetManifest`, `GetEngine` | nothing | values already in memory | none |
| `GetSettings` | nothing | the settings file | S1.2 |
| `RefreshEngine` | nothing | detection: runs the resolved Prism with `--version` | S3.1 |
| `GetInstances` | nothing | the two reads in a Prism root, and the size walk of each present instance folder | S1.1 |
| `GetPackStates` | nothing | one value of each instance's `packwiz.json`, and one bounded GET of each chapter's `pack.toml` on its allowlisted or loopback source | S1.1, S4.3 |
| `GetPrismRelease` | nothing | one bounded GET to Prism's fixed release URL | S4.5 |
| `InstallPrism` | nothing | download, verify and unpack Prism's official build | S4.5 |
| `InstallChapter` | a chapter id | the manifest's instance and pack URL: one instance folder written into the Prism root | S3.3, S4.6 |
| `LaunchChapter`, `OpenChapterWiki`, `GetServerStatus` | a chapter id | the manifest's instance, URL or address for it; before a launch, the one `PreLaunchCommand` key of the chapter's own `instance.cfg`, only from the launcher's earlier template | S3.3, S3.7, S4.6, S6.1 |
| `SetPackSource` | a chapter id and `published` or `dev` | the one `PreLaunchCommand` key of the chapter's own `instance.cfg`, between the manifest's pack URL and the loopback override from settings, only from the launcher's own template; refused while the game is active | S3.3, S4.6 |
| `StopGame` | a chapter id | the pid of the Prism the launcher started, or of the game's Java found as its child, for a run the tracker follows: asked to close, then ended | S3.3, S3.9 |
| `ShowPrismConsole` | a chapter id | the console window of the Prism the launcher started for that chapter, which the launcher's own hold hid: shown and given the foreground | S3.3, S3.7 |
| `SaveSettings` | a whole `AppSettings`, including `serverChoices`: a chapter id to the label of one of its manifest addresses, and `joinServers`: chapter ids of the manifest that have a server, validated against it | the settings file, the executable detection then runs, and, for a chapter whose chosen address moved, one ping of the manifest's address for that label; with a chapter in `joinServers`, Play passes that chapter's chosen manifest address to Prism's `--server`, checked again by `LaunchArgs` | S3.5, S6.1 |
| `ChoosePrismExecutable`, `ChoosePrismRoot` | nothing | a native file or folder picker; the pick is returned, never saved here | S3.5 |
| `OpenExternal` | a URL | the system browser, web URLs only | S3.4 |
| `GetWikiPages` | nothing | one bounded GET of the wiki's lore export on the manifest's wiki host, cached in the app data dir | S2.3, S4.3 |
| `GetWikiShots` | nothing | the screenshots the lore export lists: one bounded GET each on the manifest's wiki host, kept only when the bytes are WebP, PNG or JPEG, cached in the app data dir and served at `/wiki-art/` | S2.3, S4.3 |
| `OpenWikiPage` | a URL | the system browser, only for a URL `GetWikiPages` returned | S3.3 |
| `GetChapterSettings` | a chapter id | four keys of the chapter's own `instance.cfg` | S1.1, S3.3 |
| `SaveChapterSettings` | a chapter id and a `ChapterSettings` | five keys rewritten in the chapter's own `instance.cfg`, held to the preset list and the machine's memory | S3.2, S3.3, S4.6 |
| `GetRunLogs` | a chapter id | the names, times and sizes of `logs/latest.log`, `logs/*.log.gz` and `crash-reports/*.txt` in that chapter's instance game folder, listed through an `os.Root`; no file is opened | S3.3, S3.10 |
| `ReadRunLog` | a chapter id, a kind (`log` or `crash`), a base name the listing produces and a byte offset | one chunk (at most 256 KiB, from a whole line) of one such file in that game folder, unpacked if `.log.gz` up to 256 MiB, redacted before it leaves Go; nothing written | S3.3, S3.10, S7.5 |
| `GetChapterMods` | a chapter id | the file names, sizes and disabled state of the regular files directly in that chapter's instance `mods` folder, listed through an `os.Root`; when the game is not running, a left-over sync journal is recovered (jars in it that are still on the player's list are renamed to `.jar.disabled`) | S3.3, S3.11 |
| `SetModsDisabled` | a chapter id and a list of jar base names | the settings file (the chapter's `disabledMods`), then renames of `x.jar` and `x.jar.disabled` in that `mods` folder only, each name a plain jar name present in the folder; refused while the game runs and under a developer preview | S3.3, S3.11 |
| `WatchLiveLog` | a chapter id | the same chunk of `logs/latest.log` as `ReadRunLog` (at most 256 KiB, redacted), and a follower of that one file in that game folder, which emits what is appended as `log:live` events (at most 64 KiB each, redacted); one follower at a time, ended by the app's shutdown; nothing written | S3.3, S3.10, S7.5 |
| `StopLiveLog` | a chapter id | the follower of that chapter's log, ended; nothing else | S3.3, S3.10 |
| `CheckChapterMap` | a chapter id | one GET (5 s, no redirect followed, body dropped after 4 KiB) of the address the manifest names as that chapter's `map`, a playit tunnel with a port; nothing is kept, and a chapter with no map makes no request | S2.2, S5.4 |
| `GetPreviewSituations` | nothing | the fixed list of developer previews, in memory | none |
| `StartPreview`, `ClearPreviews` | a chapter id and a situation name, which must be one of the fixed list (`services.PreviewSituations`) | state and copy only: a synthetic `game:state`, the real loading card with a made-up report, synthetic pack, instance, engine and release answers, and a made-up Prism install that never reaches the installer. No file, process, URL or network request; a real launch, game event or write to the chapter ends the preview | S3.3, S3.8, S4.5 |

## S1. Credentials

**S1.1 The Microsoft account never enters this process.**
Holds when: nothing under `backend/` or `app.go` opens Prism's `accounts.json`;
the only account-related value is the profile *name* in
`AppSettings.ProfileName`, passed to `--profile`. Inside a Prism data
directory exactly two files are read, both in `services/instances.go`:
`prismlauncher.cfg` is scanned for the one key `InstanceDir` (every other line,
including any `ProxyPass`, is dropped unread and nothing from the file is
logged), and `<instances>/<instance id>/instance.cfg` is stat'ed once per
manifest instance id and, when present, scanned the same way for the one key
`PreLaunchCommand`, to read back the pack URL (#41). The instances folder is
never listed; a chapter's own instance folder, when present, is walked for
its size on disk (#57), and that walk takes names and sizes from the
directory entries, opens nothing, follows no symlink and stops at a ceiling.
The pack state (#71) reads one value of the instance's `packwiz.json`, the
synced `pack.toml`'s hash; the file list in it is dropped unread.
The chapter's settings (#36) read four keys of that same `instance.cfg`
(`OverrideMemory`, `MaxMemAlloc`, `OverrideJavaArgs`, `JvmArgs`) through
`scanINIKeys`, and a save reads the file whole to copy every other line back
unchanged; nothing from it is kept or logged. The writes are S4.6's: a
chapter's own instance folder, created when absent, and those five keys of
its `instance.cfg` on the player's request.
Verify: `grep -rn 'accounts\|token\|refresh' --include=*.go . | grep -v _test`;
`grep -rn 'p.open(\|os.Open\|ReadFile' backend/services/instances.go` shows two opens,
both through `scanINIKey`; `TestScanINIKeyKeepsOnlyThatKey`;
`TestInstancesSumTheInstanceFolderSize`.
Probe: a setting that points `PrismRoot` at a folder. Does anything read more
than those two files, and more than one key from each? Does the size walk
ever open a file or leave the instance folder?

**S1.2 App data holds no secrets.**
Holds when: `models.AppSettings` carries no password, token or key, and the
settings file is written `0600`. `disabledMods` (issue 156) is per chapter a list
of jar file names and nothing else.
Verify: read `backend/models/settings.go`; `TestSettingsRoundTrip` checks the mode.

## S2. The manifest

**S2.1 A manifest can name things and never run them.**
Holds when: `models.Manifest` has no field for a command, an argument, a JVM
option or a filesystem path, and `ParseManifest` refuses unknown fields.
`pack.jvm` names a preset the launcher maps to arguments (`jvmPresets`) and is
refused when unknown; the `pack.toml` URL is the only manifest value on the
pre-launch command line (`pack.toggles` names a jar-name prefix that is only
matched against file names, S3.11), and `commandSafeURL` refuses the characters Prism
reads there (`$`, quotes, spaces, a backslash, `#`).
Verify: `TestParseManifestRefusesUnknownFields`; `TestValidateManifestRefuses`;
`TestRenderInstanceConfigRefuses`; read `design/launcher.schema.json`.

**S2.2 Every manifest URL is https on an allowlisted host, a chapter's map
excepted.**
Holds when: `checkURL` runs on `wiki.baseUrl`, `pack.packwiz` and `pack.mrpack`,
and `AllowedManifestHosts` is a short fixed list. The one exception is a
chapter's optional `map` (issue 161, ADR-4's third amendment): a playit tunnel
serves BlueMap without TLS, so `checkMapURL` takes http or https on a host of
labels under `tun.ply.gg`, with an explicit port from 1 to 65535, lowercase, and
nothing after it but an optional slash (no user info, query, fragment or path).
Anything else is refused whole, and the exception reaches no other field.
Verify: `TestValidateManifestRefuses`, `TestChapterMapAcceptsOnlyATunnelWithAPort`,
`TestParseManifestRefusesABadMapAndAcceptsNoneOrNull`.
Probe: a URL with credentials, an `http://` scheme off the tunnel family, a
lookalike host (`x.tun.ply.gg.evil.example`), a missing port, `javascript:`.

**S2.3 A remote manifest is fetched, verified and cached, never executed.**
Not yet built (milestone 5). Holds when: fetched over https from the site only,
with a timeout and a size bound, parsed by the same `ParseManifest`, and the
bundled copy is the fallback on any failure.
The wiki's lore export (#58) is the first remote file and follows the same
rule: `WikiService` fetches it from the manifest's validated `wiki.baseUrl`
only, 15 s and 4 MiB, no redirects, and `parseWikiExport` keeps a page only
when its URL is a plain `/wiki/...` path (joined onto that same base), its
title and excerpt are non-empty text and it names an era; the cached copy is
parsed the same way, and the manifest's teaser is the fallback.
The export's screenshots (#141) are the first remote pictures. `parseWikiShots`
keeps an entry only when its URL is `/screenshots/<world>/<file>` of plain
characters with an image extension and no dot segment, and `fetchArt` gets each
from that same base (15 s, 4 MiB, no redirects, `If-Modified-Since` when cached)
and keeps it only when its own bytes sniff as WebP, PNG or JPEG, whatever the
server or the name says. The page never loads a remote image: Go serves the
cache at `/wiki-art/<world>/<file>` through the asset server's middleware, GET
and HEAD only, a name the same shape allows, re-sniffed, with `nosniff`, so the
CSP's `img-src 'self'` is unchanged. A file the export no longer lists is
removed from the cache on a start that reached the wiki.
Verify: `TestParseWikiShotsKeepsOnlyPlainImagePathsOfAnEra`,
`TestWikiShotsDownloadCheckCacheAndRevalidate`, `TestWikiArtMiddlewareServesOnlyCachedPictures`,
`TestWikiShotsRefuseAnOversizedPicture`,
`TestParseWikiExportKeepsOnlyPagesThePanelMayShow`,
`TestWikiPagesRefuseAnOversizedOrRedirectedExport`, `TestOpenWikiPageRefusesAnUnlistedURL`.

## S3. Paths and processes

**S3.1 Prism is run with an argument array.**
Holds when: the `exec.Command`s in the tree are Prism's (launch, `--version`,
`flatpak info`), all with argument arrays built from validated values, plus
`/usr/bin/codesign --verify` on macOS with a fixed argument list (S4.5) and
`/usr/bin/open` on macOS with one argument, the absolute path of a chapter's
instance folder (S3.6), plus the one the pre-launch sync starts, the `INST_JAVA`
Prism named with the packwiz template's arguments (S3.11); nothing invokes `sh`,
`cmd` or `powershell`. One
process is not ours: on macOS Wails' `ClipboardSetText` runs `pbcopy`, no
argument and the redacted log tail on its stdin, when the log is copied (S7.3).
Verify: the `shell never sees a command string` invariant in `.claude/suite.json`.

**S3.2 Every argument is validated.**
Holds when: `LaunchArgs` refuses an instance id that is not a plain folder
name, a server that is not `host[:port]`, a profile starting with `-`, and a
relative root.
Verify: `TestLaunchArgsRefusesAnythingThatIsNotAPlainValue`.
Probe: `--dir` as an instance id; a newline in a profile name.

**S3.3 Bridge-supplied ids resolve through the manifest.**
Holds when: `LaunchChapter`, `OpenChapterWiki` and `GetServerStatus` look the
chapter id up and use the manifest's instance id, address and path, never the
caller's. `StartPreview` (#124) does the same and takes its situation from a
fixed list, so nothing the caller sends reaches a file, a process or a URL; a
preview never writes, which `refuseUnderPreview` holds for install, pack source
and chapter settings.
Verify: read `app.go`; `TestStartPreviewRefusesWhatIsNotInTheManifestOrTheList`,
`TestWritesToAPreviewedChapterAreRefusedAndEndThePreview`,
`TestInstallPrismUnderAPreviewPlaysAFailureAndNeverInstalls`.

**S3.4 Only web URLs reach the system browser.**
Holds when: `OpenExternal` runs `services.ExternalURL` first (http, https, host
required).
Verify: `TestExternalURLAcceptsOnlyWebAddresses`.

**S3.5 A settings save can name any executable, so only the player may save.**
`SaveSettings` accepts an absolute `PrismExecutable`, and detection runs it
with `--version`; Play runs it too. That is the settings screen's purpose
(#5), so the bridge is trusted as the player here, and what keeps it the
player is S5: the WebView loads only the app's own bundled assets under a CSP
with `script-src 'self'` and no raw HTML sinks.
Holds when: `ValidateSettings` refuses a relative executable or root and a
profile starting with `-`, `SaveSettings` refuses an executable that changed
and is not an existing file, and S5.1 and S5.2 hold. The two pickers only
return what the player chose in the OS dialog; the frontend commits it through
`SaveSettings`, so a picked path and a typed one take the same route. A `packOverrides` entry
(#41) goes onto an instance's pre-launch command line, so it is held to
`CheckLocalPackURL` on save, on load (a bad entry is dropped) and again by
`InstanceCreator.Install`: plain http on `localhost`, `127.0.0.1` or `[::1]`
with a port, a path ending in `/pack.toml`, no user info, query or fragment,
and the manifest's command-line characters. A settings file cannot point a
pack at another machine.
A `serverChoices` entry (issue 151) is a chapter id and a *label*, never an
address: `ValidateSettings` holds both to their shape and `App.SaveSettings`
refuses a chapter the manifest does not have, a chapter with no server and a
label that is not in that chapter's own list (`ValidateServerChoices`), so
nothing typed on the settings screen reaches a ping or Prism's `--server`.
`services.ServerAddress` turns a label into the manifest's address, falling
back to the first when the label is missing or stale.
A `joinServers` entry (issue 163) is a chapter id and nothing more: the
player's switch for joining that chapter's server on Play, off for every chapter
until turned on. `ValidateSettings` holds each id to a chapter's shape and a
count cap, `App.SaveSettings` refuses an id that is not a chapter of the
manifest with a server (`ValidateJoinServers`), `GetSettings` prunes a stale one
(`PruneJoinServers`) and a settings file's malformed ones are dropped on load.
The address Play joins is never in settings: `joinAddress` returns
`services.ServerAddress` for a chapter in the list, else nothing, and
`LaunchArgs` validates it as it does every value that reaches Prism. The
manifest cannot turn joining on: `joinOnLaunch` is retired and a manifest that
carries it is refused whole.
Verify: `settings_test.go`, `serverchoice_test.go`, `joinservers_test.go`,
`TestSaveSettingsRefusesAnUnknownChapterOrLabelAndKeepsWhatWasSaved`,
`TestSaveSettingsRefusesAJoinSwitchForAChapterWithNoServerAndKeepsWhatWasSaved`,
`TestParseManifestRefusesTheRetiredJoinOnLaunch`; S5.1, S5.2.
Probe: anything that would put third-party script in the WebView (a remote
image or page, a manifest string rendered as HTML).

**S3.6 The folder the launcher opens is a chapter's instance folder, and nothing else.**
Holds when: `OpenInstanceFolder` takes a chapter id and resolves the folder
through `chapterInstance` (the validated manifest, the resolved Prism root, an
instance that exists); `services.OpenFolder` then refuses anything that is not
an absolute path to an existing directory. Windows opens it with `ShellExecute`
(an API call, the path is one argument), macOS with `/usr/bin/open` and that
path as its only argument, other platforms refuse.
Verify: `TestOpenInstanceFolderOpensOnlyAnInstalledChaptersFolder`,
`TestOpenFolderChecksTheDirectoryBeforeTheOSSeesIt`.
Probe: a chapter id that is a path; an instance folder that is a file or gone.

**S3.7 The launcher touches another process's window only to hide and show the game's own, and Prism's progress dialogs and console.**
Holds when: the window holder (`gamewindow_windows.go`, #45) hooks show events
of one pid, the game's Java as the tracker bound it, and acts only on a
top-level window of class `GLFW30` owned by that pid: `ShowWindow` hide and
show, and at the handover (the resource reload beginning) the show, then
`SetForegroundWindow`, then a one pixel resize and back of a window that
covers its monitor. It
reads a window's class, owner and rectangle and never its title, text or
input, injects nothing into the process (out-of-context hook), starts no
process, and is off unless the loading splash (#43) is on for the run, which
is the player's setting `loadingSplash`, on by default on Windows. A run
that ends, by any phase or by its context, releases the window.
The one other window it hides is Prism's progress dialog (#95): a top-level
window of the pid of the Prism the launcher started, whose title begins
`Please wait`, hidden with the same `ShowWindow` and shown back the same way.
The title is the whole of what is read of a window there, through
`GetWindowTextW`, kept nowhere and never logged (ADR-0012's no-title rule is
about the game's window, which has a class of its own; a Prism dialog has none
that tells it from a sign-in). Prism's sign-in, any error and a translated
Prism's dialogs do not match and stay visible, which is the safe failure. The
hold is the same `HoldWindow` as the game's, ends without showing at the
handover, and shows back what still exists when the run ends, fails or loses
its Prism first. A Prism that was already open and took the launch is not held.
The third window is Prism's console (ADR-0012, amendment): a top-level window
of that same pid whose title begins `Console window for`, hidden by the same
hook, which is not released at the handover or when the run ends but kept on a
per-chapter record until the next Play of the chapter, Stop or the launcher
quitting, and shown only by `ShowPrismConsole` (`ShowWindowAsync`, then
`SetForegroundWindow`, after the hook has ended). The prefix is matched through
`GetWindowTextW` over the first 32 characters and is never stored or logged;
only the launcher's own Prism's windows are held, by pid, and a handle is used
only while it is still a window of that pid. The held windows get `WM_CLOSE`
(the one hidden window the close routine of S3.9 reaches) and the Prism they
belong to is closed as S3.9 closes it, only while a console window is left.
Verify: `TestHolderHidesAndShowsAGLFWWindow`, `TestHolderLeavesOtherWindowsAlone`,
`TestHandoverNudgesAFullscreenWindowOnePixelAndBack`,
`TestHandoverLeavesAWindowedWindowAlone`,
`TestTrackerReleasesTheWindowWithoutForegroundWhenTheRunEndsOtherwise`;
`TestPrismDialogHoldHidesAPleaseWaitWindow`,
`TestPrismDialogHoldLeavesAnyOtherWindowOfPrismVisible`,
`TestPrismDialogReleaseAtTheHandoverLeavesThemHidden`,
`TestPrismDialogReleaseOfAFailedRunShowsThemBack`,
`TestTrackerHoldsPrismsDialogsOnlyWhileTheSplashIsOn`,
`TestTrackerShowsPrismsDialogsBackWhenTheRunEndsBeforeTheHandover`;
`TestIsPrismConsoleTitleMatchesOnlyTheConsole`,
`TestPrismConsoleHoldLeavesEveryOtherWindowOfPrismVisible`,
`TestPrismConsoleMatchIsOfThePidAndTheTitlePrefix`,
`TestPrismConsoleHoldHidesTheConsoleAndKeepsItAfterRelease`,
`TestTrackerEndsAStartFailedWhenTheConsoleAppearsBeforeTheGameLog`,
`TestTrackerLeavesAConsoleThatAppearsAfterTheGameLogBeganAlone`,
`TestTheNextPlayClosesThePrismOnTheOldConsoleAndReplacesTheHold`,
`TestShutdownEndsAPrismThatIgnoresTheCloseAndReturnsInTime`.
Probe: a window of another class, or of another process, shown while the hold
is on; a Prism window titled "Sign in" or "Error" shown during the first ten
seconds of a launch; a window titled "Minecraft Console window for" (the prefix
is at the start, or it is not a match).

**S3.8 The loading card's page is the embedded build, shows nothing from the network, and can ask Go for four things.**
Holds when: the card (#97) is a window of its own with its own webview, and the
page it loads is `splash.html` of the build embedded in the executable, served
by the host from `Page.Assets` (`splashhost.AssetsFrom`) at
`http://splash.localhost/` on Windows and `kapital-splash://app/` on macOS:
no local HTTP server, no port, nothing fetched. `AssetsFrom` serves a regular
file only, by a clean relative path (`fs.ValidPath`, so no `..`, no empty or
`.` element, no leading slash), with no backslash or NUL, and only for an
extension on its list; anything else is a 404. The page has no Wails bridge
and no bound method: it posts a string through the webview's own channel,
`splashhost.ParseMessage` accepts exactly `{"action":"leave"}`,
`{"action":"openFolder"}`, `{"action":"copyLog"}` and
`{"action":"showConsole"}` and drops, with a log
line, anything else (an unknown action, a body that is not an object, one that
is long), and a message carries no argument: the chapter is the run's own and
every path is Go's. A message from a card that has since closed is dropped.
The CSP of `splash.html` is `index.html`'s plus the `kapital-splash:` scheme
source, which only ever serves the embedded build. The state Go pushes holds
the chapter's name and pack version, the game's phase and the outcome of the
last action (a line count or an error text), and nothing about the player.
Verify: `TestAssetsRefuseAnythingOutsideTheBuild`,
`TestParseMessageAcceptsTheFourActionsOnly`,
`TestAMessageThatIsNotOneOfTheFourActionsIsDropped`,
`TestShowConsoleFromThePageShowsThePrismConsoleOfTheRunsChapter`,
`TestAMessageFromAnEarlierRunsCardChangesNothing`; `grep -c
Content-Security-Policy frontend/dist/splash.html` after a build is 1.
Probe: a page request for `../..`, `%2e%2e`, a backslash path or a `.map` file;
a posted message that names a path or another chapter.

**S3.9 Stop ends only the two processes the launcher found itself, by pid, and never by name.**
Holds when: `GameTracker.Stop` (`gametracker_stop.go`) acts on a run the
tracker follows and on nothing else: the Prism the launcher started
(`TrackRequest.Prism`, taken from the `exec.Command` that ran it) and the game's
Java the tracker found as that Prism's child (`gameRun.bound`). Each is ended
by its pid, which a run proves alive by its own exit channel and never by
looking the pid up again, and which must be above 1 (`endable`: on macOS `kill`
takes 0 for the caller's process group). No process is looked up by name to be
ended, and a Java or a Prism that was not found as ours is never touched; a
launch that went to another Prism ends the run and signals nothing. The
chapter id resolves through the manifest first (S3.3). On Windows the game's
Java is ended with `TerminateProcess` on a handle opened with
`PROCESS_TERMINATE`, and Prism is asked to close with `WM_CLOSE` posted only to
the visible top-level windows whose owner is its pid (owner and visibility are
all that is read of a window, never its title) and to the console windows the
launcher's own hold kept hidden (S3.7), then ended the same way if it is still
there five seconds later. When Stop ended the game's Java and the launcher's
Prism outlives it (waiting on the console of what it saw as a crash), the run's
end closes that Prism the same way, a second after the game, on a goroutine the
tracker's runs count, and gives up when the launcher quits (#133). A Prism left on a console is closed this way when the
chapter's next Play begins and when the launcher quits (bounded), and only while
a console window of it is left; one with a game running is not touched. On macOS it is `SIGTERM` and, five seconds
on, `SIGKILL`. The five seconds are judged on the run's steps, not slept on.
Nothing is started, read or written to do it, and the pid is logged, never a
window title.
Verify: `TestStopAsksTheLaunchersPrismToCloseAndEndsWhenItExits`,
`TestStopClosesThePrismLeftAfterTheGameAndEndsItIfItStays`,
`TestStopLeavesAPrismThatQuitsWithItsGameAlone`,
`TestAGameThatCrashedWithoutAStopLeavesPrismRunning`,
`TestStopEndsAPrismThatIgnoresTheCloseAfterFiveSeconds`,
`TestStopEndsTheGamesJavaAndTheRunEndsCrashedAsStopped`,
`TestStopWithNothingAliveEndsTheRunAtOnce`,
`TestStopNeverSignalsAPidItDoesNotOwn`,
`TestStopRefusesAChapterWithNoRunInProgress`,
`TestStopGameRefusesAnUnknownChapterAndOneWithNoRun`.
Probe: Stop on a chapter whose launch went to a Prism that was already open (no
process of ours is alive, and none may be signalled); a Prism whose pid has been
reused after it exited (its exit channel is closed, so it is not touched).

**S3.10 The logs page reads only files its own listing produces, inside the instance's game folder, and only a bounded chunk of one.**
Holds when: `GetRunLogs` and `ReadRunLog` (issue 155, ADR-2 eighth amendment)
take a chapter id, resolved through the manifest and the disk's own instance
report (S3.3), never a path. `services.ReadRunLog` takes a kind and a base name
and refuses anything but `log` or `crash` and a name that is plain
(`[A-Za-z0-9][A-Za-z0-9._-]{0,119}`, so no separator, drive, stream marker or
leading dot, and no `..`) and is `latest.log` or `*.log.gz` for a log, `*.txt`
for a crash report: the one test (`validRunLogName`) the listing also uses, so a
name that is read is a name that is listed. The file is reached through an
`os.Root` on the game folder, which refuses a path that leaves it, a link
included, and is `Lstat`ed first, so a link is refused whatever it points at and
a folder is not a file. A read returns at most 256 KiB, from a whole line; a
`.log.gz` is unpacked through a counting reader and refused past 256 MiB, so a
gzip bomb is never held (the unpack is streamed twice, to measure and to read,
and neither keeps more than the chunk). The listing caps each kind at 50. Nothing
is written, moved or deleted, and no path is returned.
Verify: `TestAReadRefusesANameThatIsNotOneTheListingProduces`,
`TestAReadRefusesWhatIsNotAFileOrHasNoGameFolder`,
`TestASymlinkOutOfTheFolderIsNeitherListedNorRead` (skipped on Windows, where a
test runner cannot make links), `TestAGzipBombBeyondTheCapIsRefused`,
`TestACorruptGzipIsAnErrorNotAPanic`, `TestListRunLogsCapsEachKindAtTheNewest`,
`TestReadRunLogRefusesWhatIsNotAListedName`.
Probe: `../../instance.cfg`, `..\..\instance.cfg`, `logs/latest.log`, a kind of
`instance`; a `latest.log` that is a link to another file; `logs` itself a link
out of the game folder; a `.log.gz` that unpacks to gigabytes.

**S3.11 The mod switches rename only plain jar files in one folder, and the pre-launch sync runs only what the pre-launch command did.**
Holds when: (a) `services.ModJarName` is the one test of a jar name, used on the
bridge's list (`ValidateDisabledMods`), on the settings file (save and load),
on every folder entry before it is listed and again before each rename: letters,
digits, space and `._+()[],'&!@-`, ending in `.jar`, no separator of either kind,
no `..`, no colon, no leading dot or space, at most 160 bytes. (b) Every rename
and removal goes through an `*os.Root` on `<game folder>/mods` opened from an
`os.Root` on the game folder, so a link that leaves either is refused by the OS
layer, and acts only on a name `Lstat`ed as a regular file: a folder or link
named like a jar is an error and untouched. (c) A manifest toggle carries only a
display name and a jar-name prefix (`jarPrefix`, `[A-Za-z0-9][A-Za-z0-9._+-]{2,63}`,
unique, none the start of another); it is matched against file names and never
reaches a path, a command or Prism. (d) The sync mode (`--prelaunch-sync`,
`services.RunSync`, ADR-2 ninth amendment) is entered only through `main()`'s
check of `os.Args`, takes exactly the flag and one URL, refuses a URL that is not
`commandSafeURL` and on the manifest allowlist or loopback (the rule an
instance's URL passes at install), reads `INST_MC_DIR`, `INST_JAVA` (both
absolute) and `INST_ID` (the instance folder pattern) from the environment,
renames mods only for an `INST_ID` that is a chapter's instance and whose game
folder is that instance's own, and runs one process: the `INST_JAVA` Prism named,
with the argument array of the packwiz template (`packwizSyncArgs`, equal to that
template token for token by `TestPackwizSyncArgsAreThePackwizCommandsArguments`),
in the game folder, no shell. It prints ASCII lines prefixed `kapital-sync:` and
reads nothing of Prism's account data. (e) A journal (`kapital-disabled.json` in
the game folder, `0600`, written atomically) is written before the first rename
and removed only when the mods are away again. (f) The launcher's copy of itself
(`SyncCopy`) is made in the data folder only, atomically, from `os.Executable`;
the path written into `instance.cfg` is held to no `$`, no `"`, no control
character and absolute (`checkSyncExePath`), and a command is the launcher's own
only when it names `<anything>/sync/kapital-launcher(.exe)` or `sync-dev`, the
flag and a URL (`classifyPreLaunch`).
Verify: `TestModJarNameAcceptsTheRealOnesAndNothingThatCouldNameAPath`,
`TestSetDisabledRefusesAFolderOrALinkNamedLikeAJar`,
`TestSetDisabledNeverFollowsALink` and
`TestModsFolderThatIsALinkOutOfTheGameFolderIsRefused` (skipped on Windows, where
a test runner cannot make links), `TestSyncRenamesNothingOutsideTheModsFolder`,
`TestSyncRefusesWhatItCannotRunSafely`,
`TestSyncRestoresRunsAndPutsTheDisabledModsAwayAgain`,
`TestSyncPutsModsAwayWhateverTheInstallerDoes`, `TestSyncAfterAKilledRun`,
`TestSetModsDisabledRefusesBeforeChangingAnything`,
`TestSetModsDisabledIsRefusedWhileTheGameRunsAndUnderAPreview`,
`TestCheckSyncExePathRefusesWhatPrismWouldReadAsSomethingElse`,
`TestValidateManifestRefusesBadToggles`; `grep -rn 'exec.Command' backend | grep -v _test`
shows `ExecSyncRunner`'s as the one new process.
Probe: a settings file with `../options.txt` or `mods/x.jar` as a name; a `mods`
folder that is a link out of the instance; a jar-named folder; INST_MC_DIR
pointing at another instance; a pack URL with `$`, a quote or another host; a
copy path with a `$` in it; a pre-launch command naming another program in a
`sync` folder.
The live log (`WatchLiveLog`, `StopLiveLog`, `services/livelog.go`) is the same
reader kept going and holds the same rules. It follows one file only, the
instance's `logs/latest.log` (the name goes through `validRunLogName` with the
kind `log`, and nothing the caller sends picks it), looked at every 500 ms
through a fresh `os.Root` on the game folder and `Lstat`ed each time, so a link
is refused and a folder is not a file. What it keeps is a byte offset and the
first 128 bytes of the file, to tell a new run's file from the one it replaced;
what it reads goes out as `log:live` events of at most 64 KiB, cut at a line
(a line still being written waits for its newline, up to one event's worth),
at most 16 events per look, every one redacted with the in-game name learned
from the log. A file that shrank or was replaced emits a reset carrying its
end. One follower runs at a time: starting one ends the one before, and
`StopLiveLog` ends the chapter's own and returns once no event can follow. It
runs under the app's run context, so quitting ends it. Nothing is written, and
no line goes to slog.
Verify: `TestAnythingButLatestLogIsNeverFollowed`,
`TestAppendedLinesArriveMaskedAndAPartialLineWaitsForItsNewline`,
`TestWhatOneLookReadsIsSentInCappedEventsWithoutLoss`,
`TestAFileThatShrankIsAResetWithTheNewFilesEnd`,
`TestStopStopsTheFollowerAndWaitsForIt`, `TestOnlyOneLogIsFollowedAtATime`,
`TestACancelledContextStopsTheFollower`, `TestALatestLogThatIsNotARegularFileIsNotRead`.
Probe: a `latest.log` swapped for a link between two looks (the root refuses
it); a game writing a line of megabytes with no newline (sent as it stands in
64 KiB pieces); two chapters' pages opened one after the other (the first is
ended before the second reads).

## S4. Downloads (milestone 4)

**S4.1 Every downloaded file is verified against its hash before use.** `.mrpack`
carries hashes; packwiz's `index.toml` does. A missing hash refuses.
packwiz-installer refused a file changed on the server without a refreshed
index on 2026-09-30 ("Hash invalid!", nothing written).
**S4.2 Archive members cannot escape.** Reject `..`, absolute paths and anything
resolving outside the instance directory.
**S4.3 Every outbound request has a timeout and a size bound.**
**S4.4 The Modrinth User-Agent is unique** and the client backs off on the
`X-Ratelimit-*` headers.

**S4.5 Prism itself is installed only on approval, and only if it verifies.**
Holds when: `ManagedPrism` reads the release from Prism's fixed repository URL;
the asset is this platform's portable build at its expected download URL with a
GitHub SHA-256 digest and a bounded size; redirects stay on GitHub's download
hosts; the digest and size are checked while streaming; unpacking refuses
escapes (S4.2) and bounds entries and bytes, and makes every folder, file and
link through an `os.Root`, so a chain of links cannot lead a write out; the
Windows executable passes `WinVerifyTrust` and the macOS bundle
`codesign --verify` (fixed arguments); and nothing is placed until all of that passed. `InstallPrism` re-reads the
release itself rather than taking one from the frontend (ADR-11). The one other
read of the installed program is its `qtlogging.ini`, bounded and refused unless
it is a `[Rules]` file, copied into the managed root only when absent
(`TestSeedLogRules*`). The approval
is the approval card's: `InstallPrism` takes no argument and holds no token,
so what Go guarantees to any caller is only that the build is Prism's own and
verified.
Verify: `managedprism_test.go` (bad digest, redirect off the allowlist, a
release URL off Prism's, failed signature, no executable, zip escapes and link
chains).
Probe: a release whose asset URL points elsewhere; a zip naming `../x`; a zip with
`a` to `.`, `b` to `a/..` and a file `b/x`.

**S4.6 A chapter's instance is created once, and only its own settings keys and its pre-launch command are ever rewritten.**
Holds when: `InstanceCreator.Create` makes the folder with a plain `mkdir` and
refuses an existing one; writes `instance.cfg` last and removes only the folder
it created on failure; fetches `pack.toml` with a timeout and a size bound, holds
any redirect to the manifest's own URL rules, and refuses versions that
disagree with the manifest; and takes the packwiz jars
only from GitHub's hosts, checked against the size and SHA-256 pinned in
`packwizjars.go`, caching nothing that fails its pin. After that,
`WriteChapterSettings` (#36, ADR-2's second amendment) rewrites five keys of
`instance.cfg` and nothing else, atomically, with the preset's arguments from
the launcher's fixed list and a memory the machine has
(`ValidateChapterSettings`), and `SaveChapterSettings` refuses while the
instance looks to be running. Before a launch, `RewritePreLaunchCommand`
(`prelaunch.go`, #95, ADR-2's fourth and ninth amendments) rewrites the one key
`PreLaunchCommand` and only when it is exactly one of the launcher's own
templates (`classifyPreLaunch`: the first, the packwiz one with the same jar
paths and flags, or the sync one naming `<dir>/sync/kapital-launcher(.exe)` or
`sync-dev`, each with a URL that passes `packURLFromCommand`), to the current
template with the same URL, atomically, every other line and its line
ending kept; anything else in the key is left alone, nothing of it is logged,
and a failed rewrite never stops the launch. It is skipped while the instance
looks to be running. On the player's request, `SwitchPackSource`
(`packswitch.go`, `SetPackSource`, ADR-2's seventh amendment) rewrites that same
key between exactly two values, the manifest's `pack.toml` and the loopback
address from `packOverrides` (`CheckLocalPackURL` again at the call), under the
same template check: the command must be exactly one of the launcher's templates
for the URL it ends in, else it is refused with an error that
names no part of it, and nothing is logged beyond the chapter and the word
`published` or `dev`. The caller supplies a chapter id and one of those two
words, never a URL. The write is atomic with every other line and its line
ending kept, and is refused while the game is active or the instance looks to be
running.
Verify: `packinstance_test.go`; `TestWriteChapterSettingsTouchesOnlyItsKeys`,
`TestRewriteINIKeysAddsMissingKeysToGeneralOnly`,
`TestValidateChapterSettingsHoldsToThePresetsAndTheMachine`,
`TestChapterSettingsRoundTripThroughTheInstance`;
`TestRewritePreLaunchCommandMovesEveryEarlierTemplateToTheCurrentOne`,
`TestRewritePreLaunchCommandLeavesWhatIsNotItsOwnAlone`,
`TestPreLaunchCommandTemplates`, `TestSyncCommandIsReadBackOnAnyOSPath`;
`TestSwitchPackSourceMovesTheURLAndNothingElse`,
`TestSwitchPackSourceRefusesWhatIsNotTheLaunchersOwnCommand`,
`TestSwitchPackSourceRefusesAURLNeitherSourceCouldHave`,
`TestSetPackSourceRefusesBeforeWritingAnything`,
`TestSetPackSourceIsRefusedWhileTheGameIsActive`.
Probe: an instance folder the player made by hand with the same name; a jar
that redirects off GitHub; a pack.toml naming two loaders; an instance whose
pre-launch command was edited by hand (an extra flag, another jar, `echo`).

## S5. WebView

**S5.1 A Content-Security-Policy ships in every build.**
Holds when: `frontend/index.html` and `frontend/splash.html` (the loading card's
page, S3.8) each carry the meta tag with `script-src 'self'`,
`frame-src 'none'`, `object-src 'none'`, and `vite.config.ts` strips it in
dev only. The one change a build makes is `index.html`'s `frame-src` (issue
161, S5.4): the `mapFrameSrc` plugin writes into it exactly the origins of the
chapters' `map` addresses in `data/launcher.json` (`scheme://host:port`, the
shape `checkMapURL` enforces, a manifest that breaks it fails the build), and
leaves `'none'` when there is none. `splash.html` frames nothing and keeps
`'none'`.
Verify: read the files; `grep -c Content-Security-Policy frontend/dist/index.html`
and the same for `splash.html` after a build are 1; `pnpm check-csp` (part of
the suite) reads the built output back and fails unless `index.html`'s
`frame-src` is the manifest's map origins and nothing wider, no other
directive carries an `http` source or a `*`, and `splash.html` says `'none'`.

**S5.2 No raw HTML sinks.**
Verify: `grep -rn 'dangerouslySetInnerHTML\|innerHTML' frontend/src` is empty.

**S5.3 Release builds have no inspector.** Wails' `Debug.OpenInspectorOnStartup`
is unset and `wails build` does not pass `-devtools`.

**S5.4 A framed map can show a page and do nothing else.**
The map page (issue 161) frames a chapter's BlueMap, a web page on the
server's own playit tunnel over plain http, inside the app's page. What is
allowed in is exactly one origin per map the bundled manifest names: the
policy's `frame-src` (S5.1), and `CheckChapterMap`'s check that the address
answers at all (S2.2 holds the address to the tunnel rule; the check follows no
redirect, reads at most 4 KiB, keeps and logs nothing of the body, and the
frame is shown only when something answered). What it can do once there:
- It cannot call a binding. Wails v2.16's `processMessage`
  (`internal/frontend/desktop/windows/frontend.go`) accepts a message only when
  both the top document's origin and the sending frame's origin are the app's
  own (`validBindingOrigin`); `BindingsAllowedOrigins` is unset in `main.go` and
  stays so. A page in a frame is on another origin, so its messages are dropped.
- It cannot take the launcher over. The `<iframe>` is sandboxed to
  `allow-scripts allow-same-origin` (BlueMap needs scripts and its own
  origin's storage): no top navigation, no popups, no forms, no pointer lock,
  and `referrerpolicy="no-referrer"`. Its origin is not the app's, so
  `allow-same-origin` gives it its own origin's storage and nothing of the app's.
- It is not trusted with anything: the address is the manifest's, never one the
  bridge sends (`CheckChapterMap` takes a chapter id), and "Open in browser"
  goes through `OpenExternal` and S3.4.
Verify: `TestCheckChapterMapAsksOnlyAboutTheManifestsAddress`,
`TestCheckMapAsksOnlyAboutAManifestShapedAddress`,
`TestProbeMapDoesNotFollowARedirect`, `TestProbeMapDropsTheBodyAfterAFewKiB`,
`pnpm check-csp`, `MapPanel.test.tsx` ("frames it sandboxed to scripts and its
own storage, with no referrer"); `grep -n BindingsAllowedOrigins main.go` is
empty.
Probe: a manifest whose map is `http://evil.example:80` or
`javascript:alert(1)` is refused whole; a map page that sends
`window.parent.postMessage` or calls `window.chrome.webview.postMessage` is
ignored by Wails.
`[verify]` on macOS (WKWebView): that Wails there gives the same guarantee for a
message from a frame (`internal/frontend/desktop/darwin/frontend.go` checks the
message's own source against the same validator, not the top document's too, so
the sender frame is what holds it), and that an http frame loads inside the
app's page (`wails://wails`) at all, since WKWebView may block it as mixed
content. The Windows side is the author's to open on the PC, in both modes.

## S6. The server ping

**S6.1 Only the manifest's addresses are pinged.**
Holds when: `services.Ping` is reached only through `StatusService.Check`,
which takes a `models.Chapter` from the validated manifest and the address
`services.ServerAddress` resolves for it (the player's saved label, looked up
in the chapter's own list, else its first address; issue 151); the frontend
passes a chapter id, never an address. An address without a port may be
redirected by that host's own `_minecraft._tcp` SRV record, and only to a
target that passes `ParseServerAddress`; anything else falls back to the
manifest's host (`TestResolveTargetFallsBackToTheHostOn25565`).
Verify: `grep -rn 'Ping(' --include=*.go . | grep -v _test`.

**S6.2 A response can change the status line and nothing else.**
Holds when: the packet length is bounded (`pingMaxResponse`), the body is
parsed as JSON into a fixed struct, and the description is flattened to text
with formatting codes stripped. Nothing from the response reaches a file, a
URL or a command.
Verify: `TestReadPacketBoundsTheLength`, `TestComponentText`.
Probe: a server that answers with a 100 MB length prefix, or never answers.

**S6.3 The ping sends nothing a server can act on.** A handshake with
protocol `-1` and an empty status request, then the connection closes.

## S7. Logs

**S7.1 The log carries no credential** and is written owner-only.
**S7.2 The share path redacts** home directory, username, server addresses and
IPv4 addresses. Verify: `TestRedactRemovesWhatIdentifiesTheUser`.
**S7.3 The copy is the redacted tail and nothing else.** `CopyRedactedLog`
(#84) reads the current log read-only, the last 256 KiB from a whole line,
drops a line still being written, masks the home path, OS user name, profile
name and every manifest server address, and writes only to the clipboard
(through Wails' `ClipboardSetText`, which on macOS pipes it to `pbcopy`).
It takes no argument from the frontend.
Verify: `TestRedactedLogTailRedacts`, `TestCopyRedactedLogPutsTheMaskedTailOnTheClipboard`.
**S7.4 The run report shows the game's log redacted, and keeps none of it.**
`GameTracker.Report` (`GetRunReport`, the card's push; ADR-2, sixth amendment)
is the one place a line of the game's `latest.log` leaves the file: the last
16 KiB, from a whole line, of the run's own log, read when the player opens the
report or when a run ends `crashed` or `failed` with the card up, never while
it runs. Holds when: (a) the tail goes through `NewRedactor` with the home
path, OS user, profile name and every manifest server address, and the report
is refused when no redactor can be built; (b) the player's in-game name,
learned from the log's own `Setting user:` line or `--username` argument (the
tail, else the head up to 8 MiB), is masked as a whole word, even with an empty
profile name; (c) IPv4 addresses, UUIDs and the values after `--accessToken`,
`--uuid`, `--username` and the like are masked; (d) the crash report is its
file name, never a path, and the file is not opened; (e) a run with no game log
of its own, or one the player stopped, shows none of a log; (f) nothing read is
stored on the tracker, written to disk or passed to `slog` (the log lines
record counts and phases only); (g) the view renders it as text, never as
HTML (S5.2), and the card's state is a JSON literal (`json.Marshal` escapes
`<`, `>`, `&`, U+2028 and U+2029), so a mod's log line cannot become script.
Verify: `TestReportMasksWhatIdentifiesThePlayerAndLearnsTheirInGameName`,
`TestReportBoundsTheLogToItsTailAndStillMasksANameOnlyTheHeadGives`,
`TestReportOfAStartThatFailedBeforeTheGameHasNoLogOrCrashReport`,
`TestReportRefusesWhatItCannotRedactOrNeverRan`,
`TestRedactMasksALaunchArgumentListsIdentityAndCredential`.
Probe: a log whose first line is a loader's argument list, and a chat line with
the player's name, in a log over 16 KiB.
**S7.5 The logs page shows a file redacted, and keeps none of it.**
`ReadRunLog` (issue 155) is the second place a line of the game's log leaves
its file, and the first for an earlier run's log, a dated `.log.gz` and a crash
report. Holds when: (a) every chunk goes through `NewRedactor` as S7.4's does
(home path, OS user, profile name, every manifest server address, IPv4
addresses, UUIDs, launch-argument values) with the in-game name added, learned
from the chunk, else the file's head up to 8 MiB, else `latest.log`'s head, and
a read is refused when there is no redactor; (b) a chunk starts and ends on a
line, so a value is not cut in two, and `latest.log`, which the game is writing,
drops a half-written last line; (c) the text is returned for the call and held by
the page alone, which empties its store when it closes, and no line of it, or of
a name learned from it, goes to `slog` (the log line is the chapter, the kind and
the number of lines); (d) the view renders it as text, never as HTML (S5.2);
(e) Copy puts on the clipboard the masked text shown, through Wails' runtime
`ClipboardSetText`, which takes no path and reads nothing.
Verify: `TestAReadMasksWhatIdentifiesThePlayer`,
`TestACrashReportsPlayerNameIsLearnedFromTheLatestLog`,
`TestTheLiveLogsHalfWrittenLastLineIsLeftOut`,
`TestRunLogsListAndReadAnInstalledChaptersFilesMasked`.
Probe: a name that appears only in the head of a log over 256 KiB, read from the
chunk before the end; a crash report from a player whose log is gone.

## S8. CI and supply chain

**S8.1 Every workflow declares top-level `permissions:`**, `contents: read` or
less.
**S8.2 Actions are pinned by commit SHA** with the version as a trailing comment.
**S8.3 Dependabot covers** gomod, npm (both projects) and github-actions.
**S8.4 A vulnerability scanner runs in CI**: `govulncheck` on both backend jobs.
**S8.5 Event text never reaches a shell through `${{ }}`.** `pr-copy.yml`,
`pr-labelled.yml` and `issue-priority.yml` read titles, bodies and labels
from `env`.
**S8.6 CodeQL and Scorecard run**, vendored verbatim from Konnekt.
`codeql.yml` scans actions, go, javascript-typescript and python on every pull
request to `main`, every push to it and weekly; `scorecard.yml` runs on every
push to `main`, on a branch protection change and weekly, and publishes its
score. Both upload to code scanning, so results sit under Security. The default
setup stays off so the two do not clash. `scorecard.yml` declares `read-all`
at the top, which S8.1 reads as read-only, and widens on its one job.
Verify: Security, Code scanning on the repository lists both tools.

## Open backlog

- S2.3, S4.2 to S4.4: not built yet; the items are written so the code meets
  them when it is.
- S5.3: verify the inspector setting against the first `wails build` output.
- S5.4: verify on macOS that a framed map cannot call a binding and that an
  http frame loads in the app's page there.
