# Spec: one Need registry

No spec baseline and no Story exist. Each requirement is traced to the #873 body and to a proposed slice in
[05-implementation-plan.md](05-implementation-plan.md). Proposed slices are not filed.

## Functional requirements

| # | Requirement | Source | Proposed slice |
|---|---|---|---|
| FR1 | The inbox kinds move into `internal/needs` with no change in behaviour. | #873 Scope: "First a move with no change in behaviour, then the new fields." | P1 |
| FR2 | Each Need kind has an area, a seat, a door, a due rule, wake rules, actions, a page key, a question, and a next action. | #873 Summary | P2 |
| FR3 | The areas are Now, Overdue, Ship, Answer, Decide, Ready, and Housekeeping (Q13). | #873 Scope | P2 |
| FR4 | Every page names a Need kind. | #873 title and Scope | P3, P4 |
| FR5 | A lint fails on a page with no kind. | #873 Scope | P3 |
| FR6 | The precision fields of V9. | #873 Scope | P5. The evidence here does not say what V9's fields are. Unverified. |
| FR7 | Deferred pages are delivered (prerequisite). | #873 Scope: "Depends on Bug #859 for deferred pages." | Bug #859 |

Bug #859 has its own Completion Goal of four criteria: a deferred page is delivered once at the first run inside
business hours, a key deferred at 22:00 is delivered at 10:00 the next working day, a failed delivery stays pending
and is retried, and an urgent page is delivered at once (#859 Completion Goal 1-4).

## Non-functional requirements

| Area | Requirement | Source |
|---|---|---|
| Reliability | The move (P1) changes no behaviour. Inferred proof: the inbox tests pass unchanged. | #873 Scope; inferred |
| Operability | Pages respect `[time] business_hours` (`Mon-Fri 09:00-18:00` by default, `internal/config/defaults.toml:214`); an urgent page ignores them (`notify.Page.Urgent`, `internal/notify/notify.go:29`). | code |
| Accessibility | Each kind has a word next to its icon, and `--plain` drops icons (`internal/cli/inbox.go`, comment above `inboxKinds`). The registry must keep this (inferred). | code; inferred |
