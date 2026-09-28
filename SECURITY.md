# Security policy

## Supported versions

Kapital Launcher is pre-1.0. Only the latest published release is supported.

## Reporting a vulnerability

**Please do not open a public issue.** Report it privately through GitHub:
**Security, then Report a vulnerability** on this repository. That opens a
thread visible only to the maintainer.

Helpful to include: what an attacker can achieve, the version and platform,
and the steps to reproduce it. Expect an acknowledgement within a few days.
There is no bounty programme.

## What is worth reporting

The launcher runs locally with the privileges of the person using it, so
"the app can read the user's files" is by design. What matters is anything
that lets **someone other than that person** influence what it does:

- **The manifest.** It comes from the Kapitel Kapital site (planned) and is
  built into the app. Anything that turns a manifest field into a command, a
  path or a download from an unexpected host is in scope.
- **Process launching.** Prism is started with an argument array built from
  validated values. An injection that turns a chapter, server address or
  profile name into an unintended argument is in scope.
- **Pack downloads**, once they exist: anything that defeats the hash check
  or lets an archive write outside the instance directory.
- **The server ping.** Anything a server response can do beyond changing the
  status line.
- **Prism's account data.** The launcher must never read it. Any path by which
  it does is in scope.

## What is out of scope

- Vulnerabilities in Minecraft, in Prism Launcher, or in mods and servers you
  chose to use. Report those to whoever maintains them.
- Anything requiring an attacker who already controls the machine.
- The absence of a hardening measure with no described impact.
