# ADR-0005: Lichdenstein server status

**Status:** proposed. Roadmap, milestone 6.

## Context

Lichdenstein is a server plus a client visuals pack; "Play" joins it. The
action bar's state line wants to say whether the server is up and how many
are on. The Minecraft Server List Ping is the documented way to ask, and it
reveals the user's IP only to the author's own server.

## Proposal

- Ping the manifest's `server.address` and nothing else, from Go, with a
  short timeout, on chapter open and on demand; never a third-party host.
- Show online state, player count and MOTD in the state line.
- **Offline behaviour is the author's call** and is the open question: disable
  Join, or still launch the visuals pack so the user can join later from
  inside the game. The handoff asks it; nothing here answers it.

## Verify before building

The current protocol (handshake, status request, JSON response) against the
minecraft.wiki page for Server List Ping, and the target server's Minecraft
version, which is itself still `[PLACEHOLDER]` in the manifest.
