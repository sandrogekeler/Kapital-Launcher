---
paths:
  - "go.mod"
  - "go.sum"
  - "frontend/package.json"
  - "frontend/pnpm-lock.yaml"
  - "package.json"
---

# Before adding a dependency

`agent_docs/DEPENDENCIES.md` is the policy and the inventory: one line of
rationale per direct dependency and a record of what was considered and not
added. Read it first and record the addition there in the same pull request.

The short version: Go prefers the standard library, since this app spawns a
process on the user's machine and every dependency is trusted with that; npm
prefers what is already in the tree, lazy-loads anything heavy, and
`pnpm check-bundle` holds the entry chunk to its budget.

Wails is pinned by hand and excluded from Dependabot: a minor bump changes the
generated bindings and the WebView it links against, so it is a deliberate
piece of work, not a routine bump.
