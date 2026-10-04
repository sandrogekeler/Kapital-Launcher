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

