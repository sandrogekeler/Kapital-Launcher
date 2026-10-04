**NOT AN OFFICIAL MINECRAFT PRODUCT. NOT APPROVED BY OR ASSOCIATED WITH MOJANG OR MICROSOFT.**

Kapital Launcher needs [Prism Launcher](https://prismlauncher.org/), which does the sign-in and the launching. The app finds it, and offers to download it when it is missing; nothing is fetched until you approve.

These builds are not code-signed, so Windows and macOS both warn before the first run. That is expected, and the steps below get past it.

## Windows 11

Download `Kapital-Launcher-<version>-windows-amd64-setup.exe` and run it. If SmartScreen shows "Windows protected your PC", choose "More info", then "Run anyway". The setup installs for your user only, so it asks for no administrator rights, adds Kapital Launcher to the Start menu and the desktop, and installs Microsoft's WebView2 runtime if it is missing. Uninstalling from Windows' settings leaves your settings and worlds in place.

`Kapital-Launcher-<version>-windows-amd64.exe` is the same app without the setup: it runs from wherever you put it.

## macOS 12 or later

Download `Kapital-Launcher-<version>-macos-universal.dmg` (Apple Silicon and Intel), open it and drag Kapital Launcher onto Applications. The first time you open it, macOS says it cannot verify the app. Open System Settings, go to Privacy & Security, scroll to the Security section, choose "Open Anyway" for Kapital Launcher, then choose Open and confirm with your password. This is needed once.

## Verifying a download

Each file carries a build attestation that ties it to the workflow run in this repository that produced it. With the [GitHub CLI](https://cli.github.com/):

```
gh attestation verify <file> -R sandrogekeler/Kapital-Launcher
```

`checksums.txt` lists the SHA-256 of every file: check it with `sha256sum -c checksums.txt` (Linux, Git Bash) or `shasum -a 256 -c checksums.txt` (macOS), or compare `Get-FileHash <file>` in PowerShell with the matching line.

## Code signing policy

Free code signing provided by [SignPath.io](https://about.signpath.io), certificate by [SignPath Foundation](https://signpath.org). This covers the Windows builds. The first beta was published before signing was set up and is not signed.

Every release is built from the public source by the project's release workflow on GitHub, and only those builds are signed.

- Committers and reviewers: [Alessandro Gekeler](https://github.com/sandrogekeler)
- Approvers: [Alessandro Gekeler](https://github.com/sandrogekeler)

Privacy policy: this program will not transfer any information to other networked systems unless specifically requested by the user or the person installing or operating it. It has no telemetry and no account of its own. To show what it shows, it reads the Kapitel Kapital wiki, asks each chapter's game server whether it is online, checks the pack host for a newer pack and GitHub for a newer Prism Launcher. It downloads Prism Launcher and a chapter's pack only when you ask it to. Signing in to Minecraft is done by Prism Launcher with Microsoft, under their own terms.

To remove it: on Windows, uninstall Kapital Launcher from Settings, Apps; on macOS, move it from Applications to the Trash. Your settings and worlds stay until you delete them yourself.
