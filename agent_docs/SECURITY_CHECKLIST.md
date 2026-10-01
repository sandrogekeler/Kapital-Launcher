# Kapital Launcher: Security Checklist

How each security-relevant part of the app must be, how to verify it, and what
to try to break. The target, not a snapshot: an item that turns out wrong is
corrected here with the reason. Only `Open backlog` should churn.

**Who the attacker is:** anyone other than the person running the launcher.
Two reaches matter: **network** (a manifest, a pack index, a download, a server
ping response) and **bridge** (a bound method on `App`, callable by anything
that runs in the WebView).

Bound methods on 2026-09-30: **21** (`grep -c '^func (a \*App) [A-Z]' app.go`).
A different count is new surface to classify: add the method to this table.

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
| `SaveSettings` | a whole `AppSettings` | the settings file, and the executable detection then runs | S3.5 |
| `ChoosePrismExecutable`, `ChoosePrismRoot` | nothing | a native file or folder picker; the pick is returned, never saved here | S3.5 |
| `OpenExternal` | a URL | the system browser, web URLs only | S3.4 |
| `GetWikiPages` | nothing | one bounded GET of the wiki's lore export on the manifest's wiki host, cached in the app data dir | S2.3, S4.3 |
| `OpenWikiPage` | a URL | the system browser, only for a URL `GetWikiPages` returned | S3.3 |
| `GetChapterSettings` | a chapter id | four keys of the chapter's own `instance.cfg` | S1.1, S3.3 |
| `SaveChapterSettings` | a chapter id and a `ChapterSettings` | five keys rewritten in the chapter's own `instance.cfg`, held to the preset list and the machine's memory | S3.2, S3.3, S4.6 |

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
settings file is written `0600`.
Verify: read `backend/models/settings.go`; `TestSettingsRoundTrip` checks the mode.

## S2. The manifest

**S2.1 A manifest can name things and never run them.**
Holds when: `models.Manifest` has no field for a command, an argument, a JVM
option or a filesystem path, and `ParseManifest` refuses unknown fields.
`pack.jvm` names a preset the launcher maps to arguments (`jvmPresets`) and is
refused when unknown; the `pack.toml` URL is the only manifest value on the
pre-launch command line, and `commandSafeURL` refuses the characters Prism
reads there (`$`, quotes, spaces, a backslash, `#`).
Verify: `TestParseManifestRefusesUnknownFields`; `TestValidateManifestRefuses`;
`TestRenderInstanceConfigRefuses`; read `design/launcher.schema.json`.

**S2.2 Every manifest URL is https on an allowlisted host.**
Holds when: `checkURL` runs on `wiki.baseUrl`, `pack.packwiz` and `pack.mrpack`,
and `AllowedManifestHosts` is a short fixed list.
Verify: `TestValidateManifestRefuses`.
Probe: a URL with credentials, an `http://` scheme, a lookalike host.

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
Verify: `TestParseWikiExportKeepsOnlyPagesThePanelMayShow`,
`TestWikiPagesRefuseAnOversizedOrRedirectedExport`, `TestOpenWikiPageRefusesAnUnlistedURL`.

## S3. Paths and processes

**S3.1 Prism is run with an argument array.**
Holds when: the `exec.Command`s in the tree are Prism's (launch, `--version`,
`flatpak info`), all with argument arrays built from validated values, plus
`/usr/bin/codesign --verify` on macOS with a fixed argument list (S4.5) and
`/usr/bin/open` on macOS with one argument, the absolute path of a chapter's
instance folder (S3.6); nothing invokes `sh`, `cmd` or `powershell`.
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
caller's.
Verify: read `app.go`.

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
Verify: `settings_test.go`; S5.1, S5.2.
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

**S3.7 The launcher touches another process's window only to hide and show the game's own, and Prism's progress dialogs.**
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
Verify: `TestHolderHidesAndShowsAGLFWWindow`, `TestHolderLeavesOtherWindowsAlone`,
`TestHandoverNudgesAFullscreenWindowOnePixelAndBack`,
`TestHandoverLeavesAWindowedWindowAlone`,
`TestTrackerReleasesTheWindowWithoutForegroundWhenTheRunEndsOtherwise`;
`TestPrismDialogHoldHidesAPleaseWaitWindow`,
`TestPrismDialogHoldLeavesAnyOtherWindowOfPrismVisible`,
`TestPrismDialogReleaseAtTheHandoverLeavesThemHidden`,
`TestPrismDialogReleaseOfAFailedRunShowsThemBack`,
`TestTrackerHoldsPrismsDialogsOnlyWhileTheSplashIsOn`,
`TestTrackerShowsPrismsDialogsBackWhenTheRunEndsBeforeTheHandover`.
Probe: a window of another class, or of another process, shown while the hold
is on; a Prism window titled "Sign in" or "Error" shown during the first ten
seconds of a launch.

**S3.8 The loading card's page is the embedded build, shows nothing from the network, and can ask Go for three things.**
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
`{"action":"openFolder"}` and `{"action":"copyLog"}` and drops, with a log
line, anything else (an unknown action, a body that is not an object, one that
is long), and a message carries no argument: the chapter is the run's own and
every path is Go's. A message from a card that has since closed is dropped.
The CSP of `splash.html` is `index.html`'s plus the `kapital-splash:` scheme
source, which only ever serves the embedded build. The state Go pushes holds
the chapter's name and pack version, the game's phase and the outcome of the
last action (a line count or an error text), and nothing about the player.
Verify: `TestAssetsRefuseAnythingOutsideTheBuild`,
`TestParseMessageAcceptsTheThreeActionsOnly`,
`TestAMessageThatIsNotOneOfTheThreeActionsIsDropped`,
`TestAMessageFromAnEarlierRunsCardChangesNothing`; `grep -c
Content-Security-Policy frontend/dist/splash.html` after a build is 1.
Probe: a page request for `../..`, `%2e%2e`, a backslash path or a `.map` file;
a posted message that names a path or another chapter.

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
escapes (S4.2) and bounds entries and bytes; the Windows executable passes
`WinVerifyTrust` and the macOS bundle `codesign --verify` (fixed arguments);
and nothing is placed until all of that passed. `InstallPrism` re-reads the
release itself rather than taking one from the frontend (ADR-11). The approval
is the approval card's: `InstallPrism` takes no argument and holds no token,
so what Go guarantees to any caller is only that the build is Prism's own and
verified.
Verify: `managedprism_test.go` (bad digest, redirect off the allowlist, a
release URL off Prism's, failed signature, no executable, zip escapes).
Probe: a release whose asset URL points elsewhere; a zip naming `../x`.

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
(`prelaunch.go`, #95, ADR-2's fourth amendment) rewrites the one key
`PreLaunchCommand` and only when it is exactly the launcher's earlier template
(the same jar paths and flags, a URL that passes `packURLFromCommand`), to the
current template with the same URL, atomically, every other line and its line
ending kept; anything else in the key is left alone, nothing of it is logged,
and a failed rewrite never stops the launch. It is skipped while the instance
looks to be running.
Verify: `packinstance_test.go`; `TestWriteChapterSettingsTouchesOnlyItsKeys`,
`TestRewriteINIKeysAddsMissingKeysToGeneralOnly`,
`TestValidateChapterSettingsHoldsToThePresetsAndTheMachine`,
`TestChapterSettingsRoundTripThroughTheInstance`;
`TestRewritePreLaunchCommandMovesTheEarlierTemplateToTheCurrentOne`,
`TestRewritePreLaunchCommandLeavesWhatIsNotItsOwnAlone`,
`TestPreLaunchCommandRunsThePackSyncHeadless`.
Probe: an instance folder the player made by hand with the same name; a jar
that redirects off GitHub; a pack.toml naming two loaders; an instance whose
pre-launch command was edited by hand (an extra flag, another jar, `echo`).

## S5. WebView

**S5.1 A Content-Security-Policy ships in every build.**
Holds when: `frontend/index.html` and `frontend/splash.html` (the loading card's
page, S3.8) each carry the meta tag with `script-src 'self'`,
`frame-src 'none'`, `object-src 'none'`, and `vite.config.ts` strips it in
dev only.
Verify: read the files; `grep -c Content-Security-Policy frontend/dist/index.html`
and the same for `splash.html` after a build are 1.

**S5.2 No raw HTML sinks.**
Verify: `grep -rn 'dangerouslySetInnerHTML\|innerHTML' frontend/src` is empty.

**S5.3 Release builds have no inspector.** Wails' `Debug.OpenInspectorOnStartup`
is unset and `wails build` does not pass `-devtools`.

## S6. The server ping

**S6.1 Only the manifest's addresses are pinged.**
Holds when: `services.Ping` is reached only through `StatusService.Check`,
which takes a `models.Chapter` from the validated manifest; the frontend
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
name and every manifest server address, and writes only to the clipboard.
It takes no argument from the frontend.
Verify: `TestRedactedLogTailRedacts`, `TestCopyRedactedLogPutsTheMaskedTailOnTheClipboard`.

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
