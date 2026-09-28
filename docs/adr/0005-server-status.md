# ADR-0005: Server status

**Status:** accepted, 2026-09-28. Built.

## Context

Lichdenstein is a server plus a client visuals pack; Frangfurd is a modpack
that also has a server. Both want the action bar to say whether the server
is up and how many are on. The Java Edition Server List Ping is the
documented way to ask (minecraft.wiki, read 2026-09-28), and it reveals the
user's IP only to the author's own servers.

## Decision

- **Every chapter with a `server` is pinged**, by Go, once a minute while the
  app is open and on demand (opening the chapter, the refresh control). The
  manifest's `server.address` is the only address ever asked;
  `services.Ping` takes nothing else, sends a handshake with protocol `-1`
  and an empty status request, and reads one bounded response.
- **Results are events.** `StatusService.Run` emits `server:status` with the
  chapter id on every result; the frontend's `useServerStore` listens and
  files each under its chapter. Nothing in the frontend polls.
- **Play is not disabled when the server is offline.** The line says
  "Server offline · as of 12:34" and the button stays live: a server-first
  chapter still launches its visuals pack, and Prism's own connect screen
  says what happened. The question the handoff asked is answered that way
  because the alternative hides a working pack behind a probe that can be
  wrong (a firewall, a restart in progress).
- **Joining is a manifest choice**, `server.joinOnLaunch`: true for
  Lichdenstein, whose Play is Join; false for Frangfurd, which is played as
  a pack and joined from inside the game. The button's verb follows it.
- The facts panel shows what the server itself reports (`version.name`)
  over what the manifest says it runs, once a ping has answered.

## Consequences

- A hostile or broken server can change the status line and nothing else:
  the response is size-bounded, parsed as JSON, and its description is
  flattened to plain text with formatting codes stripped.
- The addresses are `placeholder.invalid` until the real ones are known,
  which reads as offline. That is correct.
- `.invalid` never resolves, so the ticker's first pass on a fresh build
  logs one refused connection per server chapter per minute. Fine until
  the addresses land; if it becomes noise, the log line drops to debug.
