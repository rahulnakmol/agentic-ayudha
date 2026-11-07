# Implementation plan: the answer reaches the asking session

## Stories and Bugs

Feature #881 has no Stories or Bugs (`_data/subs.tsv`). The rows below are proposed, not filed. No source estimates effort
or risk.

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| P1 | Each answer delivery has a receipt: sent, pull-pending, read, or failed | Story (proposed, not filed) | not filed | not estimated | not estimated | #880 (answer record) | `internal/cli/ask.go`, `internal/delegate/state.go` (inferred) | not started |
| P2 | An answer is pushed to the asking cloud session through the Amp, Claude cloud, Codex cloud, or Cursor adapter | Story (proposed, not filed) | not filed | not estimated | not estimated | P1, #871 (session on the ask) | `internal/delegate/receivers.go`, `internal/cli/ask.go` | not started |
| P3 | A local session waits for its answer with `aayu ask --wait` | Story (proposed, not filed) | not filed | not estimated | not estimated | P1 | `internal/cli/ask.go`, `internal/verbs/registry.go` (the flag), generated files | not started |
| P4 | Each host's agent learns to ask and wait: `skills/ask/SKILL.md` and the host files | Story (proposed, not filed) | not filed | not estimated | not estimated | P2, P3 | `skills/ask/SKILL.md`, `hosts/**`, `internal/hosts/**`, `internal/cli/install.go` | not started |
| P5 | The inbox shows each answer's receipt | Story (proposed, not filed) | not filed | not estimated | not estimated | P1, #873 | `internal/cli/inbox.go` or `internal/needs/**` | not started |

## Delivered

Nothing under #881. The push channel exists: four adapters with `FollowUp` (before v0.3.0, for review fixes), and
mission event delivery through it, in
[7a9f3d7](https://github.com/rahulnakmol/agentic-ayudha/commit/7a9f3d7e) (#1097, Story #1008 of Feature #889).

## In review

None (`_data/open-prs.tsv`).

## Pending

- P1 to P5.
- Person steps: #881's baselines and Approve to build (none on the ledger).
- How aayu learns "read": no source says. Unverified.
- Moved out to v0.4.1: Copilot, the session-start hook, and the Claude Notification hook (#881 Scope, Out).
