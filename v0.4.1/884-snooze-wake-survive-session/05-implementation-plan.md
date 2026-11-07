# #884 Implementation plan

## Stories and Bugs

No Story or Bug is filed under [#884](https://github.com/rahulnakmol/agentic-ayudha/issues/884). The P rows are
proposed, not filed; effort and risk are blank because no source estimates them. #939 is filed under Feature #975
and is listed for reference.

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| P1 | A skip or snooze is a line in `inbox/<login>.jsonl` on the ledger, and the inbox honours it (proposed, not filed) | Story | not filed | — | — | — | L02 `internal/cli/inboxtui.go`, `internal/cli/inbox.go`, a new `internal/cli/inboxsnooze.go`; L07 `internal/config/**`; L04 `CONSTRAINTS.md` | not started |
| P2 | A snoozed card wakes at its time or when its item changes (proposed, not filed) | Story | not filed | — | — | P1 | L02 `internal/cli/inboxsnooze.go`, `internal/cli/inbox.go` | not started |
| P3 | A question snoozed past due needs a reason, and the seat is told by comment (proposed, not filed) | Story | not filed | — | — | P1, #883, #880 | L02 `internal/cli/inboxsnooze.go`, `internal/cli/inboxtui.go` | not started |
| P4 | Ask someone: a mention or `aayu counsel --post`, and the With others area (proposed, not filed) | Story | not filed | — | — | P1 | L02 `internal/cli/inboxtui.go`, a new `internal/cli/inboxothers.go` | not started |
| [#939](https://github.com/rahulnakmol/agentic-ayudha/issues/939) | Not now, snooze, and not sure: put a card away and see when it comes back (under #975, reference only) | Story | planned | M | low | #884, #937 (#939 Links) | not stated in this review | not started |

## Delivered

Nothing of #884 is delivered. The session-only skip in `internal/cli/inboxtui.go` exists from before this Feature
(commit not checked here).

## In review

None. No open PR refers to #884.

## Pending

- P1 to P4, after the baselines and Approve to build.
- Overlap to settle: #939 under #975 designs the same menus. A person decides which Feature owns the TUI work.
- No source gives an estimate.
