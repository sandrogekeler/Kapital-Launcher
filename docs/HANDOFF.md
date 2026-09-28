# Kapital Launcher — Build Handoff

> Hand this to a fresh Claude Code session together with `kapital-launcher-design-reference.html` (the visual reference).
> Written 2026-09-28. Items marked **[verified]** were checked against a source on that date (links at the end). Items marked **[verify]** are assumptions the new session must confirm before relying on them.

---

## 1. What we're building

A personal desktop launcher for the **Kapitel Kapital** Minecraft packs. It is a branded front-end, **not** a from-scratch launcher. Prism Launcher does the heavy lifting: Microsoft sign-in, Java, mod loaders, and launching.

| Chapter | Type | Accent | Notes |
|---|---|---|---|
| 01 Luxemburg | Modpack, creative building expansion | `#e0aa4e` | |
| 02 Lichdenstein | Vanilla-plus **server** + client visual-mods pack | `#5cbfa2` | "Play" = launch visuals pack and join server |
| 03 Frangfurd | Modpack, modern creative expansion | `#7cbfe0` | NeoForge 1.21.1 |

- **Audience:** personal use, and possibly friends on the same servers. Not a public product.
- **Name:** Kapital Launcher.
- **Brand source:** the Kapitel Kapital site repo `sandrogekeler/kapitel-kapital-wiki`. Its `src/styles/tokens.css` holds the neutrals and fonts, and `public/` holds the fonts, wordmark and screenshots.

### Why not a from-scratch launcher
Microsoft/Minecraft sign-in needs an Azure app ID that Microsoft must approve for the Minecraft Services API. Solo developers in 2026 report being refused and redirected to the Xbox developer programme **[verified]**. Prism already has approved access **[verified]**. Using Prism avoids the entire auth problem.

---

## 2. Decisions already made (don't reopen without reason)

1. **Engine = Prism Launcher.** It must be installed separately, and the user signs in to Microsoft **inside Prism**. Kapital Launcher never handles Microsoft credentials.
2. **Integration = Prism's documented CLI** **[verified]**:
   - `-l / --launch <instanceId>` launches an instance. The instance ID is its folder name.
   - `-s / --server <address>` joins a server on launch (only together with `--launch`).
   - `-a / --profile <name>` picks the account (only together with `--launch`).
   - `-d / --dir <path>` sets a custom Prism root directory.
   - `-I / --import <zip or URL>` imports an instance.
   - `--show <instanceId>` opens that instance's window.
   - `--alive` writes a `live.check` file after Prism starts.
3. **Chapter accents are canonical as in the table above.** The website currently has them wrong (see §9).
4. **Design:** follow the reference HTML. That means the dark neutral base, Fraunces for display, Inter for UI, JetBrains Mono for data, one accent per chapter, restrained motion, and no gradients or emoji.

---

## 3. Architecture decisions still to make

Write each one as a short ADR in `docs/adr/` before building on it.

### ADR-1 · App framework
| Option | For | Against |
|---|---|---|
| **Wails v2** (Go + React) | Sandro already built Konnekt with it; stable | v3 is the future |
| **Wails v3** | Newer API | Still **Beta** as of the 25 Sep 2026 status page **[verified]**; APIs may still change |
| Tauri (Rust + web) | Small binaries, mature updater | New language; no existing code to reuse |

**Recommendation:** Wails v2, to reuse Konnekt patterns. Reconsider v3 once it reaches GA. Check the current Wails v2 version at build time **[verify]**.

### ADR-2 · How Kapital Launcher owns Prism's data
- **A) Use the user's normal Prism install and data folder.** Simple, but instance names can collide with the user's own instances.
- **B) A dedicated Prism root via `--dir`** (for example `%APPDATA%/KapitalLauncher/prism`). This isolates Kapital instances and accounts. The trade-off is that the user signs in to Microsoft once more, inside that root.
- **Recommendation:** B. Confirm that `--dir` combined with `--launch` behaves as expected on all target OSes **[verify]**.
- Also decide whether to **detect** an existing Prism install or **guide the user to install it**. Do not silently download or bundle it; see §5 on the GPL.

### ADR-3 · Pack format and distribution
- **packwiz** (MIT **[verified]**) keeps a git-friendly TOML source of each pack and can export Modrinth `.mrpack` files **[verified]**. `packwiz-installer` gives auto-updating MultiMC-family instances **[verified]**.
- **Option A:** publish a `.mrpack` per version and import it with `prismlauncher -I <url>`.
- **Option B:** set Prism's pre-launch command to run `packwiz-installer` against a hosted `pack.toml`, so the pack syncs on every launch. Prism exposes pre-launch, post-exit and wrapper commands **[verified]**.
- **Recommendation:** packwiz as the source of truth. Use option B for day-to-day updates and option A for a fresh install. Confirm how Prism's import handles an existing instance with the same name **[verify]**.
- **Hosting:** a static host such as Cloudflare Pages/R2 or GitHub Releases. It only serves the pack manifests and `index.toml`; the mods themselves download from Modrinth or CurseForge.

### ADR-4 · Where launcher metadata comes from
The chapter list, blurbs, screenshots, changelog and server address need a source:
- **Option 1:** a `launcher.json` published by the Kapitel Kapital site at build time. Then there is one source of truth, and the wiki blurbs are already there.
- **Option 2:** a separate repo or manifest.
- **Recommendation:** Option 1. Add an endpoint to the Astro site.

### ADR-5 · Lichdenstein server status
- Query the server with the Minecraft **Server List Ping** protocol to get online status and player count. It is a documented protocol, but check the current spec on minecraft.wiki **[verify]**.
- Decide what happens when the server is offline: disable "Join", or still launch the visuals pack?

### ADR-6 · Launcher self-update and code signing
- Choose a self-update mechanism, or stay manual at first.
- Unsigned Windows builds trigger SmartScreen warnings and unsigned macOS builds hit Gatekeeper. Decide whether that is acceptable for personal use.

### ADR-7 · Target OSes
Windows only, or also macOS/Linux? This affects Prism path detection and packaging.

---

## 4. Foundations to install

| Tool | Purpose | Notes |
|---|---|---|
| Go | Wails backend | Use the version current Wails v2 requires **[verify]** (Wails v3 needs Go 1.25+ **[verified]**) |
| Node + npm | React frontend | |
| Wails CLI | Scaffolding and builds | `wails doctor` checks the platform prerequisites |
| WebView2 (Windows) | Wails runtime | Usually already present on Windows 10/11 |
| Prism Launcher | Engine | Current release 11.1.0 (3 Sep 2026) **[verified]** |
| packwiz (CLI) | Pack authoring | MIT **[verified]** |
| packwiz-installer (+ bootstrap jar) | Pack sync at launch | Java; runs inside Prism's pre-launch step |
| Fonts | Fraunces, Inter, JetBrains Mono | Copy the woff2 files from the site repo's `public/fonts/` |

---

## 5. Licences to check

1. **Prism Launcher: GPL-3.0** **[verified]**.
   - Calling it as a separate program through its CLI is generally treated as "aggregation", not a derived work. **[verify]** this reading; it isn't legal advice.
   - Forking Prism's code, or shipping a modified Prism, puts your code under the GPL and has further consequences. A fork most likely also cannot reuse Prism's Microsoft client ID **[verify]**.
   - Bundling an **unmodified** Prism installer still carries GPL duties: you must provide or point to its source and include the licence text.
2. **Mods: check each mod's licence and distribution permission.**
   - `.mrpack` and packwiz *reference* download URLs instead of re-hosting jars. Keep it that way, and never commit mod jars to your own hosting.
   - CurseForge-hosted mods can opt out of third-party distribution, in which case they need a manual download **[verify]** the current API behaviour. Prefer Modrinth sources wherever a mod is on both.
3. **Minecraft Usage Guidelines** **[verified]**:
   - Don't use "Minecraft" as the primary name. "Kapital Launcher" is fine.
   - Don't use Minecraft logos or trademark-style lettering.
   - Show the disclaimer in the app (for example on the About screen) and on any download page: `NOT AN OFFICIAL MINECRAFT PRODUCT. NOT APPROVED BY OR ASSOCIATED WITH MOJANG OR MICROSOFT.`
4. **Fonts:** Fraunces, Inter and JetBrains Mono are believed to be SIL OFL. **[verify]** each one and ship its licence files.
5. **Your own assets** (the wordmark and screenshots) are yours. Keep attribution consistent with the site.
6. **Modrinth API:** usage must comply with the Modrinth Terms of Service **[verified]**.

---

## 6. Privacy and safety

- **Microsoft credentials:** never read, copy or log Prism's account data or tokens. Kapital Launcher only passes a profile *name* to `--profile`.
- **Modrinth API:** send a unique `User-Agent` such as `sandrogekeler/kapital-launcher/<version> (<contact>)`; generic user agents risk being blocked **[verified]**.
  - The rate limit is 300 requests per minute per IP **[verified]**. Cache responses and back off when the `X-Ratelimit-*` headers run low.
- **Download integrity:** verify every downloaded file against its hash (`.mrpack` includes hashes; packwiz uses `index.toml` hashes). Reject any file whose hash doesn't match.
- **Remote manifests are untrusted input:**
  - Only fetch over HTTPS from hosts on a fixed allowlist.
  - Never let a manifest set Prism pre-launch commands, JVM arguments or arbitrary shell commands. Build those locally from a template.
  - Guard against path traversal in `.mrpack` overrides and instance names: reject `..`, absolute paths, and anything that resolves outside the instance directory.
  - Optionally sign the `launcher.json` manifest.
- **Command execution:**
  - Call Prism with an argument array, never a shell string, so a crafted instance name or server address can't inject a command.
  - Validate server addresses as `host[:port]`.
- **Telemetry:** none. The only outbound traffic should be to your own hosts, Modrinth/CurseForge, and whatever Prism itself contacts.
- **Logs:** game and Prism logs can contain the Minecraft username, file paths and server IPs. Add a "copy log" action that redacts these before anything gets shared.
- **Server ping:** the Server List Ping only reveals the user's IP to your own server, which is fine. Just don't ping third-party hosts.

---

## 7. Suggested first milestones

1. Scaffold with Wails v2 + React + TypeScript, and apply the design tokens from the reference HTML.
2. Detect Prism: find the binary on each OS, read its version, and point the user to prismlauncher.org if it's missing.
3. Launch an existing instance end to end: `prismlauncher -d <root> -l <id>`, plus `-s <addr>` for Lichdenstein.
4. Author the Frangfurd pack in packwiz and host it; import it into Prism and sync it on launch via packwiz-installer.
5. Build the `launcher.json` endpoint on the site and render the chapter list from it.
6. Add Lichdenstein server status.
7. Polish: changelog panel, error states, redacted log export, About screen with the disclaimer.

---

## 8. Open questions for Sandro

- Which OSes are needed?
- Will friends use it, or only Sandro? This changes how much signing, onboarding and hosting matter.
- Lichdenstein: which Minecraft version, and which visual mods (shaders? Iris or Oculus?)
- Luxemburg: which loader and Minecraft version?
- Where should packs be hosted?

---

## 9. Side fix: the website's chapter accents are wrong

In `kapitel-kapital-wiki/src/lib/taxonomy.mjs`, update `ERAS[].accent` to match the canonical colours:
- Luxemburg `brass` is already correct (`#e0aa4e`).
- Lichdenstein: change `ice` to `verdigris` (`#5cbfa2`).
- Frangfurd: change `oxide` to `ice` (`#7cbfe0`).

The site repo also has no Lichdenstein screenshots yet (`public/screenshots/lichdenstein/` is empty). The launcher needs at least one.

---

## 10. Claude Code working notes

- Put this file in the new repo as `docs/HANDOFF.md`.
- Keep `CLAUDE.md` short: build commands, stack, and the "never touch Microsoft tokens" and "argument-array only" rules, with a link to this file. Don't paste the whole handoff into it.
- Use plan mode for the first scaffold, and record ADR outcomes in `docs/adr/` as they're decided.

---

## Sources
- Prism Launcher CLI — https://prismlauncher.org/wiki/getting-started/command-line-interface/
- Prism custom commands (pre-launch, post-exit, wrapper) — https://prismlauncher.org/wiki/help-pages/custom-commands/
- Prism releases (11.1.0) — https://prismlauncher.org/news/tag/Release/
- Prism GPL-3.0 and features — https://space-node.net/blog/prism-launcher-setup-guide-2026
- Microsoft auth / Azure app approval (Minecraft Wiki) — https://minecraft.wiki/w/Microsoft_authentication
- Microsoft Q&A, XboxLive.signin for individual launchers — https://learn.microsoft.com/en-us/answers/questions/5971906/xboxlive-signin-minecraft-services-access-for-an-i
- Microsoft Q&A, Xbox Developer Program requirement — https://learn.microsoft.com/en-gb/answers/questions/5768276/how-to-get-xboxlive-signin-permission-for-azure-ap
- Wails v3 status — https://v3.wails.io/status/
- packwiz — https://github.com/packwiz/packwiz
- Modrinth API — https://docs.modrinth.com/api/
- Minecraft Usage Guidelines — https://www.minecraft.net/en-us/usage-guidelines
