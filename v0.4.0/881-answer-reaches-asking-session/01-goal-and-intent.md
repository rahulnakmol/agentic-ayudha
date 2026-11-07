# Goal and intent: the answer reaches the asking session

No intent baseline exists (`specs/881/` is absent on `origin/ledger`). The source is the #881 body.

## Goal

"aayu delivers each answer to the session that asked: a push to a cloud session, and `aayu ask --wait` for a local
session." (#881 Summary).

## Who it serves and the outcome

- The agent session that asked through `aayu ask` (#880): it gets the answer without a person copying it.
- The person: a receipt says whether the answer was sent, waits for a pull, was read, or failed (#881 Scope).

## Problem

"`Adapter.FollowUp` sends only review fixes for delegated builds (`internal/delegate/receivers.go:48-52`), with no
receipt (G3, Q14)." (#881 Problem). On `dev` at 884d81b, part of this has changed: `FollowUp` now also carries a
mission's Kanha brief (`internal/cli/kanha.go:78`), event lines to a Kanha (`internal/cli/kanha.go:169`,
`internal/cli/missiontick.go:300` at bc78349 (`:282` at 884d81b)), and a stop line to a story thread (`internal/cli/missionstop.go:488`). The review
fix path is `internal/cli/delegate.go:1146`. "No receipt" still holds: a Kanha event records only "sent" or "not
sent: …" in the delegation's events (`internal/cli/kanha.go:155-176`).

## Success measures

None is named in the issue.

## Out of scope

"Copilot; the session-start hook (Claude Code, Codex) and the Claude Notification hook for a local session (narrowed
2026-10-28, v0.4.1)." (#881 Scope, Out).
