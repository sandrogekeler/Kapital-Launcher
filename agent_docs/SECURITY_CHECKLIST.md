# Kapital Launcher: Security Checklist

How each security-relevant part of the app must be, how to verify it, and what
to try to break. The target, not a snapshot: an item that turns out wrong is
corrected here with the reason. Only `Open backlog` should churn.

**Who the attacker is:** anyone other than the person running the launcher.
Two reaches matter: **network** (a manifest, a pack index, a download, a server
ping response) and **bridge** (a bound method on `App`, callable by anything
that runs in the WebView).

Bound methods on 2026-09-28: **9** (`grep -c '^func (a \*App) [A-Z]' app.go`).
A different count is new surface to classify.

## S1. Credentials

**S1.1 The Microsoft account never enters this process.**
Holds when: nothing under `backend/` or `app.go` opens Prism's `accounts.json`
or anything under its data directory; the only account-related value is the
profile *name* in `AppSettings.ProfileName`, passed to `--profile`.
Verify: `grep -rn 'accounts\|token\|refresh' --include=*.go . | grep -v _test`.
Probe: a setting that points `PrismRoot` at a folder. Does anything read it?

**S1.2 App data holds no secrets.**
Holds when: `models.AppSettings` carries no password, token or key, and the
settings file is written `0600`.
Verify: read `backend/models/settings.go`; `TestSettingsRoundTrip` checks the mode.

## S2. The manifest

**S2.1 A manifest can name things and never run them.**
Holds when: `models.Manifest` has no field for a command, an argument, a JVM
option or a filesystem path, and `ParseManifest` refuses unknown fields.
Verify: `TestParseManifestRefusesUnknownFields`; read `design/launcher.schema.json`.

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
Holds when: the only `exec.Command` in the tree takes the executable and
`LaunchArgs`' output; nothing invokes `sh`, `cmd` or `powershell`.
Verify: the `shell never sees a command string` invariant in `.claude/suite.json`.

**S3.2 Every argument is validated.**
Holds when: `LaunchArgs` refuses an instance id that is not a plain folder
name, a server that is not `host[:port]`, a profile starting with `-`, and a
relative root.
Verify: `TestLaunchArgsRefusesAnythingThatIsNotAPlainValue`.
Probe: `--dir` as an instance id; a newline in a profile name.

**S3.3 Bridge-supplied ids resolve through the manifest.**
Holds when: `LaunchChapter` and `OpenChapterWiki` look the chapter id up and
use the manifest's instance id, address and path, never the caller's.
Verify: read `app.go`.

**S3.4 Only web URLs reach the system browser.**
Holds when: `OpenExternal` runs `services.ExternalURL` first (http, https, host
required).
Verify: `TestExternalURLAcceptsOnlyWebAddresses`.

## S4. Downloads (milestone 4)

**S4.1 Every downloaded file is verified against its hash before use.** `.mrpack`
carries hashes; packwiz's `index.toml` does. A missing hash refuses.
**S4.2 Archive members cannot escape.** Reject `..`, absolute paths and anything
resolving outside the instance directory.
**S4.3 Every outbound request has a timeout and a size bound.**
**S4.4 The Modrinth User-Agent is unique** and the client backs off on the
`X-Ratelimit-*` headers.

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

## S6. Logs

**S6.1 The log carries no credential** and is written owner-only.
**S6.2 The share path redacts** home directory, username, server addresses and
IPv4 addresses. Verify: `TestRedactRemovesWhatIdentifiesTheUser`.

## S7. CI and supply chain

**S7.1 Every workflow declares top-level `permissions:`** with `contents: read`.
**S7.2 Actions are pinned by commit SHA** with the version as a trailing comment.
**S7.3 Dependabot covers** gomod, npm (both projects) and github-actions.
**S7.4 A vulnerability scanner runs in CI**: `govulncheck` on the backend job.

## Open backlog

- S2.3, S4: not built yet; the items are written so the code meets them when it is.
- S5.3: verify the inspector setting against the first `wails build` output.
