See @agent_docs/CLAUDE.md for the stack, conventions and build commands.

Two rules hold before any file is opened:

- **Never touch Microsoft credentials.** Prism owns the sign-in. This app passes a
  profile *name* to `--profile` and never reads, copies or logs Prism's account data.
- **Argument arrays only.** Prism is run with `exec.Command(exe, args...)`. No shell,
  no command string, anywhere.

`docs/HANDOFF.md` is the brief this project was started from; `docs/adr/` records
what was decided since. Read `agent_docs/ROADMAP.md` before adding a feature.
