# ADR-0011: Getting Prism for the player

**Status:** accepted, 2026-09-30. Built (#23): the service, the approval card
in the action bar and the update offer on the engine card.

## Context

Kapital Launcher drives Prism Launcher and cannot work without it. Until now a
player without Prism saw "Prism not found" and a link to prismlauncher.org,
then had to install Prism, get through its setup wizard and come back. The
author asked for the launcher to get Prism itself, on approval, and for Prism
to stay out of sight: no Prism window except where Prism must be seen.

The handoff put "bundling or modifying Prism" out of scope: GPL-3.0 duties,
and a fork cannot reuse Prism's Microsoft client id. Downloading Prism's own,
unmodified release at the player's request is neither: the launcher ships no
Prism code, and the player gets exactly what prismlauncher.org offers.

Read on 2026-09-30 from Prism 11.1.1's source and release:

- Every release publishes portable builds, `PrismLauncher-Windows-MSVC-Portable-<v>.zip`
  (and an arm64 one) and `PrismLauncher-macOS-<v>.zip`, and GitHub publishes a
  SHA-256 digest for every release asset.
- Prism's Windows executables are Authenticode-signed (11.1.0 and 11.1.1,
  verified with `WinVerifyTrust`).
- `Application::createSetupWizard` shows the setup wizard when the language is
  unset, Java is neither found nor downloaded automatically, a theme is
  invalid, a legacy paste URL is set, or no Microsoft account is signed in.
- `--launch` without `--show-window` starts a game without Prism's main
  window.
- Prism's updater is enabled whenever `prismlauncher_updater.exe` sits beside
  the executable (the program folder, not the data root) and reads
  `auto_check` (default on) from `prismlauncher_update.cfg` in the data root.

## Decision

- **A launcher-managed portable Prism, on approval only.** When no Prism is
  found, the launcher offers to get it, showing the version, size, source and
  licence. Nothing is downloaded before the player agrees.
- **Players who have Prism keep theirs.** Detection tries the player's own
  Prism first (settings, PATH, standard locations, Flatpak) and the managed
  copy last. ADR-2 is unchanged for them.
- **Verified before use.** The release is read from Prism's own repository
  (fixed URL); the asset must be this platform's portable build, at its
  expected download URL, with a SHA-256 digest and a sane size. The download
  follows redirects only to GitHub's download hosts, is size-bounded and
  hashed while it streams, and is refused on any mismatch. It ends when no
  bytes arrive for a minute, not after a fixed time, so a slow connection
  still finishes. Unpacking refuses
  entries outside the target, rooted names and symlinks leaving the folder,
  and bounds entries and bytes. Windows then requires a valid Authenticode
  signature (`WinVerifyTrust`, no shell); macOS runs `codesign --verify --deep
  --strict` with a fixed argument array and, since #226, a requirement that
  the signer is Prism's Developer ID team (see the amendment below).
- **Program and data apart.** `<data>/prism/app-<version>/` is the program,
  replaced whole on update (the installed version is never reinstalled, and a
  leftover folder is moved aside before it is removed, so a running Prism is
  never deleted in place); `<data>/prism/root/` is Prism's data root, passed
  as `--dir` on every launch and never touched by an update. The managed copy
  is the dedicated root ADR-2 describes, used here because there is no root of
  the player's to share. The portable build's `portable.txt` is removed:
  with it, Prism opened by hand would keep its data in the program folder,
  which the next update replaces; without it, it uses Prism's usual data
  folder.
- **Prism never asks what the launcher can answer.** Before Prism first
  starts, the launcher writes `prismlauncher.cfg` (language, theme, automatic
  Java download and switching, quit after the game stops) and
  `prismlauncher_update.cfg` (`auto_check=false`), each only if absent, so a
  player's later changes stay theirs. What remains of the wizard is the
  Microsoft sign-in, which is Prism's and stays Prism's.
- **A third seeded file: the log rules.** The launcher ends a start that fails
  before the game by following Prism's log for the failed `LaunchTask` line
  (#103, #106). Prism's own `qtlogging.ini` (beside the program on Windows, in
  the bundle's resources on macOS) says `launcher.task=false`, which silences
  that category at every level, Critical included, so on a stock Prism the line
  is never written. Prism loads the first `qtlogging.ini` it finds, data root
  first, and only that one, so the managed root gets a copy of Prism's file,
  verbatim, with `launcher.task.critical=true` added last (later rules win in
  Qt). It is a copy and never a subset, so rules such as
  `launcher.auth.credentials.debug=false` stay in force. It is written at
  install and, when absent, before each launch of the managed Prism, so an
  earlier install gets it on its next Play; an existing file is the player's.
  A source that is missing, over a size bound or not a `[Rules]` file writes
  nothing and is logged once, and the launch goes on. `[verify]` on a real Mac:
  that the file is at `Prism Launcher.app/Contents/Resources/qtlogging.ini`.
- **Updates on a click.** On start the launcher reads Prism's latest release;
  when it is newer than the managed copy, and the managed copy is the Prism
  in use, it offers the update, through the same verification. Never silent.

## Consequences

- A player without Prism goes from nothing to playing with one approval and
  one Microsoft sign-in, and never sees Prism's setup.
- The launcher now makes outbound requests to `api.github.com` and GitHub's
  download hosts, but only when checking for or installing Prism.
- A managed Prism is not the player's regular Prism: opening it outside the
  launcher needs `--dir`. The launcher is its only front door.
- Observed on Windows 11: the managed 11.1.1 installs, verifies and starts with
  its data root and language as seeded, opening "Prism Launcher Quick Setup".
  That the sign-in is its only page follows from the source above; the page
  itself was not inspected. The updater's `auto_check=false` could not be
  seen taking effect, because the updater is created with the main window,
  after the sign-in.
- `[verify]` on a real Mac: that Prism's macOS zip is signed and passes
  `codesign`, that its app bundle's symlinks unpack intact, and where
  `prismlauncher_update.cfg` applies (the Mac build updates through Sparkle).

## Amendment, 2026-10-09: who signed it (#226)

`codesign --verify` alone checks that a signature is intact, not whose it is:
an ad hoc signature passed. The macOS runners read Prism 11.1.1's signer (#220):
Developer ID Application, Sefa Eyeoglu, Team ID `MZM5U2NVNH`, notarized. The
check now passes `-R` with the designated-requirement shape of a Developer ID
app for that team (`prismRequirement` in `verify_darwin.go`). If Prism ever
changes signer, installs and updates are refused, with an error naming the
Team ID, until the launcher is updated: a deliberate trade for not accepting
any signer. Windows still accepts any signature `WinVerifyTrust` trusts; pinning
Prism's Authenticode signer there is a separate change.
