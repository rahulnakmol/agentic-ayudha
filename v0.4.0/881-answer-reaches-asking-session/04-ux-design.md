# UX design: the answer reaches the asking session

Status: planned, not built. No UX baseline exists.

## The agent

| Session | Planned experience | Source |
|---|---|---|
| Cloud (Amp, Claude cloud, Codex cloud, Cursor) | The answer arrives as a message in the asking thread or task. | #881 Scope |
| Local | The agent runs `aayu ask --wait` and gets the answer when it comes. | #881 Summary |
| Copilot, and local sessions through hooks | Not in v0.4.0; moved to v0.4.1. | #881 Scope, Out |

The message text and the `--wait` output are not specified. A related message that exists today: a Kanha gets
`answered: #<n> resumes` from `Env.answeredAsk` once #880 calls it (`internal/cli/kanha.go:203`).

## The person

- The inbox shows each delivery's receipt: sent, pull-pending, read, or failed (#881 Scope; `internal/cli/inbox.go` in
  Expected paths). The card layout is not specified.
- A skill, `skills/ask/SKILL.md`, tells each host's agent how to ask and wait (#881 Expected paths). Its content is
  not specified.
