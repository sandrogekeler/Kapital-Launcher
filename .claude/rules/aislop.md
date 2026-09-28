---
paths:
  - ".aislop/**"
  - ".aislopignore"
  - ".github/workflows/aislop.yml"
---

# The aislop gate

`scanaislop/aislop` (MIT, run through `npx`, nothing installed) scores the tree
for what AI-assisted code leaves behind: ignored errors, hidden fallbacks,
duplicated blocks, oversized functions. CI runs `aislop ci` and fails below
100, through `.github/workflows/aislop.yml`, vendored from Kollektiv, which
pins aislop 0.16.0 and ruff 0.16.7 once for the whole suite.

**ruff has to be present or aislop's Python engines silently run nothing**, and
the pinned version is what the gate judges against. The vendored check runner
knows this: `.claude/suite-check.py` skips the aislop command, never passes
it, when ruff is missing or on another version. Locally, put ruff 0.16.7 on
`PATH` (a venv is fine) before running it.

`.aislop/base.yml` is the suite's policy, vendored and never edited here.
`.aislop/config.yml` extends it (`extends: ./base.yml`, and the `./` is
load-bearing) and holds only this tree's size limits, a **ratchet** held at the
largest function and file. Today both sit at the tool's defaults; lower them
if the tree shrinks below that, never raise them to make a build pass.

`.aislopignore` lists what is generated or vendored and never scored.

A finding that is a documented exception gets an inline
`// aislop-ignore-next-line <rule> -- <reason>` beside it, the way
`lib/ipc.ts`'s sanctioned no-bridge fallback does. A bare directive with no
reason is the thing to refuse in review.

**Never run `aislop fix`**: it deletes lines by regex and rewrites
`package.json`. Telemetry is off in the config.
