# Kapital Launcher: Security Checklist

How each security-relevant part of the app must be, how to verify it, and what
to try to break. The target, not a snapshot: an item that turns out wrong is
corrected here with the reason. Only `Open backlog` should churn.

**Who the attacker is:** anyone other than the person running the launcher.
Two reaches matter: **network** (a manifest, a pack index, a download, a server
ping response) and **bridge** (a bound method on `App`, callable by anything
that runs in the WebView).

Bound methods on 2026-09-30: **14** (`grep -c '^func (a \*App) [A-Z]' app.go`).
A different count is new surface to classify: add the method to this table.

| Method | Takes from the bridge | Reaches | Item |
|---|---|---|---|
| `GetAppVersion`, `GetManifest`, `GetEngine` | nothing | values already in memory | none |
| `GetSettings` | nothing | the settings file | S1.2 |
| `RefreshEngine` | nothing | detection: runs the resolved Prism with `--version` | S3.1 |
| `GetInstances` | nothing | the two reads in a Prism root | S1.1 |
| `GetPrismRelease` | nothing | one bounded GET to Prism's fixed release URL | S4.5 |
| `InstallPrism` | nothing | download, verify and unpack Prism's official build | S4.5 |
| `InstallChapter` | a chapter id | the manifest's instance and pack URL: one instance folder written into the Prism root | S3.3, S4.6 |
| `LaunchChapter`, `OpenChapterWiki`, `GetServerStatus` | a chapter id | the manifest's instance, URL or address for it | S3.3, S6.1 |
| `SaveSettings` | a whole `AppSettings` | the settings file, and the executable detection then runs | S3.5 |
| `OpenExternal` | a URL | the system browser, web URLs only | S3.4 |

## S1. Credentials

**S1.1 The Microsoft account never enters this process.**
Holds when: nothing under `backend/` or `app.go` opens Prism's `accounts.json`;
the only account-related value is the profile *name* in
`AppSettings.ProfileName`, passed to `--profile`. Inside a Prism data
directory exactly two reads happen, both in `services/instances.go`:
`prismlauncher.cfg` is scanned for the one key `InstanceDir` (every other line,
including any `ProxyPass`, is dropped unread and nothing from the file is
logged), and `<instances>/<instance id>/instance.cfg` is stat'ed, once per
manifest instance id. The instances folder is never listed. The one write is
S4.6's: a chapter's own instance folder, created when absent.
Verify: `grep -rn 'accounts\|token\|refresh' --include=*.go . | grep -v _test`;
`grep -rn 'p.open(\|os.Open\|ReadFile' backend/services/instances.go` shows one open;
`TestScanInstanceDirKeepsOnlyThatKey`.
Probe: a setting that points `PrismRoot` at a folder. Does anything read more
than those two paths from it?

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

## S3. Paths and processes

**S3.1 Prism is run with an argument array.**
Holds when: the `exec.Command`s in the tree are Prism's (launch, `--version`,
`flatpak info`), all with argument arrays built from validated values, plus
`/usr/bin/codesign --verify` on macOS with a fixed argument list (S4.5);
nothing invokes `sh`, `cmd` or `powershell`.
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
profile starting with `-`, and S5.1 and S5.2 hold.
Verify: `settings_test.go`; S5.1, S5.2.
Probe: anything that would put third-party script in the WebView (a remote
image or page, a manifest string rendered as HTML).

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

**S4.6 A chapter's instance is created once and never overwritten.**
Holds when: `InstanceCreator.Create` makes the folder with a plain `mkdir` and
refuses an existing one; writes `instance.cfg` last and removes only the folder
it created on failure; fetches `pack.toml` with a timeout and a size bound, holds
any redirect to the manifest's own URL rules, and refuses versions that
disagree with the manifest; and takes the packwiz jars
only from GitHub's hosts, checked against the size and SHA-256 pinned in
`packwizjars.go`, caching nothing that fails its pin.
Verify: `packinstance_test.go`.
Probe: an instance folder the player made by hand with the same name; a jar
that redirects off GitHub; a pack.toml naming two loaders.

## S5. WebView

**S5.1 A Content-Security-Policy ships in every build.**
Holds when: `frontend/index.html` carries the meta tag with `script-src 'self'`,
`frame-src 'none'`, `object-src 'none'`, and `vite.config.ts` strips it in
dev only.
Verify: read both files; `grep -c Content-Security-Policy frontend/dist/index.html`
after a build is 1.

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

## S8. CI and supply chain

**S8.1 Every workflow declares top-level `permissions:`**, `contents: read` or
less.
**S8.2 Actions are pinned by commit SHA** with the version as a trailing comment.
**S8.3 Dependabot covers** gomod, npm (both projects) and github-actions.
**S8.4 A vulnerability scanner runs in CI**: `govulncheck` on both backend jobs.
**S8.5 Event text never reaches a shell through `${{ }}`.** `pr-copy.yml`,
`pr-labelled.yml` and `issue-priority.yml` read titles, bodies and labels
from `env`.

CodeQL and Scorecard, which the Kollektiv suite vendors, are not here: both
need a public repository (or Advanced Security) and this one is private. Add
them the day it is not.

## Open backlog

- S2.3, S4.2 to S4.4: not built yet; the items are written so the code meets
  them when it is.
- S5.3: verify the inspector setting against the first `wails build` output.
