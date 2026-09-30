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
