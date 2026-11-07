# #883 Implementation plan

## Stories and Bugs

No Story or Bug is filed under [#883](https://github.com/rahulnakmol/agentic-ayudha/issues/883). The P rows are
proposed, not filed; effort and risk are blank because no source estimates them. #940 and #859 are filed elsewhere
and listed for reference.

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| P1 | Each open question gets the Decision SLA's deadlines (proposed, not filed) | Story | not filed | — | — | #880, #873 | L03 `internal/decide/**`; L07 `internal/config/**` if a key is added | not started |
| P2 | The sweep walks questions and pages once at 24 h (proposed, not filed) | Story | not filed | — | — | P1; overlaps Bug #859 | L02 `internal/cli/decide.go`; L03 `loops/decisions.toml` (only if the loop changes) | not started |
| P3 | A two-way question takes the recommendation at 48 h, shown in Decide to ratify or revert (proposed, not filed) | Story | not filed | — | — | P2 | L02 `internal/cli/decide.go`; L03 `internal/decide/**` | not started |
| P4 | The Overdue area and banner come first in the inbox (proposed, not filed) | Story | not filed | — | — | P1 | L02 `internal/cli/inbox.go`, `internal/cli/inboxtui.go` | not started |
| [#940](https://github.com/rahulnakmol/agentic-ayudha/issues/940) | Overdue at 24 hours, and a recommendation applies at 48 hours only for a two-way ask (under #975, reference only) | Story | planned | S | medium | #883, #937 (#940 Links) | not stated in this review | not started |
| [#859](https://github.com/rahulnakmol/agentic-ayudha/issues/859) | Deferred pages are never sent, and their key is never paged again (v0.4.0, reference only) | Bug | planned | S | low | — | not stated in this review | not started |

## Delivered

Nothing of #883 is delivered. The Decision SLA it extends exists from before v0.3.0 (`internal/decide/decide.go`,
`internal/cli/decide.go`; commit not checked here).

## In review

None. No open PR refers to #883.

## Pending

- P1 to P4, after the baselines and Approve to build.
- Blocking prerequisite: Feature #880 must define what a question is. It has no Stories.
- Overlap to settle: #940 under #975 designs the Overdue area; P4 and #940 must not both build it.
- Dependent: Story #1106 (v0.4.1) needs this SLA for its open questions.
- No source gives an estimate.
