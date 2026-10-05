# ADR-0013: The player's profile from Mojang

**Status:** accepted, 2026-10-05. Built (issue 193).

## Context

The account page (issue 192) holds one account value, the profile name the
launcher passes to Prism's `--profile`, and shows it as initials. Players know
themselves by their skin, and a mistyped name is only found out when Play
starts Prism on the wrong account. Mojang answers both questions from public
endpoints that need no sign-in and no token: who has this name, and what does
their skin look like.

This is the first time the launcher sends something about the player off the
machine, so it is an architecture decision and not a feature detail. The
author decided on 2026-10-04 to build it in its own pull request with this
ADR, and that only the profile name leaves the machine.

## Decision

- **Three new outbound hosts**, all asked by Go and never by the page, in this
  order for one lookup:
  1. `api.mojang.com/users/profiles/minecraft/<name>` gives the UUID and
     Mojang's own spelling of the name. 404 and 204 mean no profile has it.
  2. `sessionserver.mojang.com/session/minecraft/profile/<uuid>` gives the
     profile's `textures` property, which names the skin's address.
  3. `textures.minecraft.net/texture/<hash>` gives the skin PNG.
- **What leaves the machine.** The profile name, in the path of the first
  request, and only after it matches Minecraft's own rule
  (`^[A-Za-z0-9_]{3,16}$`). Then the UUID that request returned, in the
  second. The third carries a hash Mojang named. The User-Agent is the
  launcher's own, as every other fetcher's. No cookie, no token, no
  credential of any kind is sent or read, and nothing of Prism's accounts is
  touched: this is the public profile and not the sign-in (CLAUDE.md).
- **What is kept**, in a folder of its own, `mojang/`, in the app data dir: the
  UUID, Mojang's spelling of the name and when it was asked (`lookup.json`),
  and the face as a 64x64 PNG (`<uuid>.png`). One profile at a time: a lookup
  for another name replaces both, and a name Mojang does not know removes
  both. The skin is decoded in memory and dropped, no other field of any
  response is kept, and the log carries the outcome (`found`, `not_found`,
  `unknown`) and never the name, the UUID or an address.
- **The face.** Go cuts the 8x8 face at (8,8) from the skin, lays the hat layer
  at (40,8) over it where it has alpha, and scales it by nearest neighbour.
  The skin is read only as a PNG of 64x64 or 64x32 (the size is read before
  the pixels), at most 256 KiB, whatever the server's headers say. An old
  64x32 skin whose hat area is opaque throughout has no hat, as in the game.
- **The skin's address is held to a shape.** The session server writes it as
  `http://textures.minecraft.net/texture/<hash>`. Go accepts the exact host
  `textures.minecraft.net`, no port, credentials or query, a lower-case hex
  hash, and then requests `https://textures.minecraft.net/texture/<hash>`
  itself: the scheme Mojang wrote is never followed. Any other host or path
  is refused and the profile simply has no face.
- **Bounds on every request.** 8 s each, no redirect followed, the bodies read
  to a cap (4 KiB, 32 KiB, 256 KiB) and refused above it.
- **The page never loads a remote image.** Go serves the face at
  `/mojang-face/<uuid>.png` through the asset server's middleware, GET and
  HEAD only, one name shape, read through an `os.Root`, re-sniffed as PNG, with
  `nosniff`. The CSP is unchanged (`img-src 'self'`).
- **The cache.** A lookup and its face are used for 24 hours, then asked again
  the next time the account page opens or the name is saved. When Mojang
  cannot be reached, the older lookup and face still answer, so the page
  looks the same offline.
- **A quiet fallback.** Offline, a timeout, a rate limit or an unexpected
  answer is the status `unknown`; no profile with the name is `not_found`; a
  name that cannot be one is `invalid` and nothing is asked. None is an
  error. The page keeps the initials, says nothing about `unknown` and
  `invalid` (the field already warns about the second), and says "No
  Minecraft profile has this name" only for `not_found`.
- **It is on, not opt-in.** There is no setting. The lookup runs only on the
  account page, a page the player opens on purpose, for a name the player typed
  there, and the name is what Prism is told to sign in as: it is a public
  Minecraft name, not a secret, and the request is the same one every
  launcher and server list makes. An opt-in would hide a convenience behind a
  switch for a request that carries no more than the name the game itself
  sends to Mojang at every start. If that judgement turns out wrong, the
  switch is one setting and one early return in `GetPlayerProfile`.
- **When it asks.** When the page opens and after the name is saved, never per
  keystroke, and never for the empty name (Prism's default account).
- **The UUID is copied by Go**, through the same clipboard call as the log copy
  (`CopyPlayerUUID`), and only the UUID of the latest found profile: the page
  sends no text to copy. "Change skin" opens Mojang's own page through the
  existing `OpenExternal` check.

## Consequences

- The roadmap's "no telemetry" line gains Mojang's public profile lookup in the
  outbound list, and `SECURITY_CHECKLIST.md` gains the two bound methods and an
  item for the fetch.
- Mojang learns that this IP asked about this name, as it does from any
  client. The launcher says so here and nowhere sends more.
- The lookup says only whether a name is a profile. It does not say the
  player owns it, is signed in to it, or that Prism has an account for it, and
  the page does not claim any of those.
- A name that is a profile but whose skin cannot be read keeps its UUID and the
  initials. A skin hosted anywhere but `textures.minecraft.net` is never
  fetched, so a change of Mojang's hosting shows as a missing face until this
  ADR is amended.
- Mojang's endpoints are rate limited. A day's cache and a lookup only on the
  account page keep the launcher far below it, and a refusal is `unknown`.
