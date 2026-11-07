# Implementation plan: one Need registry

## Stories and Bugs

Feature #873 has no Stories or Bugs (`_data/subs.tsv`). Rows P1 to P5 are proposed, not filed; they follow the order the #873 Scope sets ("First a move with no change in behaviour, then the new fields"). The call-site work is split by file
group so that lanes do not share files. No source estimates effort or risk for the proposed slices. Bug #859 is a
filed prerequisite outside the Feature.

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| [#859](https://github.com/rahulnakmol/agentic-ayudha/issues/859) | Deferred pages are never sent, and their key is never paged again | Bug (prerequisite; not a sub-issue of #873) | planned | S | low | none | L06, L02: `internal/notify/notify.go`, a caller of `Sender.Flush` | not started; no PR in `_data/open-prs.tsv` |
| P1 | The inbox kinds move into `internal/needs`, with no change in behaviour | Story (proposed, not filed) | not filed | not estimated | not estimated | none | `internal/needs/**` (new), `internal/verbs/registry.go` (layer glob), `internal/cli/inbox.go`, `inboxtui.go`, `inboxaudit.go`, `inboxready.go`, `inboxwhy.go`, `inboxwork.go` | not started |
| P2 | Each Need kind has an area, door, due rule, wake rules, actions, page key, question, and next action, and the inbox groups by area | Story (proposed, not filed) | not filed | not estimated | not estimated | P1 | `internal/needs/**`, `internal/cli/inbox.go`, `internal/cli/inboxtui.go` | not started |
| P3 | A page names its Need kind, and a lint fails on a page with no kind | Story (proposed, not filed) | not filed | not estimated | not estimated | P1, #859 | `internal/notify/**`, a lint test (inferred: `internal/needs/` test file) | not started |
| P4a | Build and delegate pages name their kind | Story (proposed, not filed) | not filed | not estimated | not estimated | P2, P3 | `internal/cli/build.go`, `internal/cli/delegate.go` | not started |
| P4b | Halt, resume, status digest, and streak pages name their kind | Story (proposed, not filed) | not filed | not estimated | not estimated | P2, P3 | `internal/cli/halt.go`, `internal/cli/status.go`, `internal/cli/streak.go` | not started |
| P4c | Decision SLA and inspect pages name their kind | Story (proposed, not filed) | not filed | not estimated | not estimated | P2, P3 | `internal/cli/decide.go`, `internal/cli/inspect.go`, `internal/cli/inspect_app.go`, `internal/inspect/inspect.go`, `internal/cli/scope.go` | not started |
| P4d | The budget breaker's page names its kind | Story (proposed, not filed) | not filed | not estimated | not estimated | P2, P3 | `internal/models/gateway.go` | not started |
| P5 | The precision fields of V9 | Story (proposed, not filed) | not filed | not estimated | not estimated | P2 | `internal/needs/**` (inferred) | not started; V9 is unverified |

Bug #859's effort and risk come from its labels (`effort:S`, `risk:low`; #859 JSON labels). `internal/cli/scope.go` is in #873 Expected paths but has no page call site today; it is placed in P4c as inferred.

## Delivered

Nothing under #873. Since v0.3.0, other Stories added kinds to the hard-coded list, which grows the move: for
example Story #1172 (`baseline`), Story #1173 (`approve`), Story #1001 (`mission`), Story #1013 (`mission-start`,
`locked`), Story #1015 (`mission-time`), Story #1176 (`evening`), and Story #1178 (`overnight`, bc78349) (the
comments on each constant, `internal/cli/inbox.go:46-63`).

## In review

None (`_data/open-prs.tsv`).

## Pending

- Bug #859 and P1 to P5.
- Person steps: #873's baselines and Approve to build (none on the ledger). #859 is a Bug: it needs a triage
  decision before Ready (Decision #861 title: "every Bug needs a triage decision, before Ready").
- V9's precision fields: not defined in any source read here. Unverified.
