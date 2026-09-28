# ADR-0002: Which Prism data root the launcher uses

**Status:** accepted, 2026-09-28. **Deviates from the handoff's recommendation;
read the trade-off before relying on it.**

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
- The launcher never writes into a Prism root itself. Instances arrive through
  Prism's own import (`-I`), milestone 4.

## Consequences

- Zero setup for someone who already has Prism signed in: install, open, Play.
- A user who already has an instance called `kapital-luxemburg` collides; that
  is the case the setting exists for.
- `--dir` combined with `--launch` is still `[verify]` on every OS, as the
  handoff asked; milestone 3 does it against a real instance.
- Reopen if instance management (milestone 4) turns out to need to write into
  the root directly: a dedicated root would then be the safer default.
