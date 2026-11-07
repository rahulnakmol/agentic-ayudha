# #884 Spec

No spec baseline exists (`specs/884` is absent from the ledger) and no Story is filed under #884. Requirements trace
to #884 and to Story #939's Completion Goal, which sits under Feature #975. Slices are proposed, not filed (see
[05-implementation-plan.md](05-implementation-plan.md)).

## Functional requirements

| # | Requirement | Source | Slice |
|---|---|---|---|
| FR1 | A snooze is a line in `inbox/<login>.jsonl` on the ledger, so every surface and machine reads the same state. | [#884](https://github.com/rahulnakmol/agentic-ayudha/issues/884) Summary and Scope | P1 |
| FR2 | A snoozed card wakes at its time, or sooner on a change of the item. | #884 Scope | P2 |
| FR3 | On a Ship card, `s` offers "1 it changes · 2 09:00 tomorrow · 3 Monday 09:00"; 3 prints "Skipped #825 until Monday 09:00. It comes back sooner if its head, checks, or comments change." | [#939](https://github.com/rahulnakmol/agentic-ayudha/issues/939) criteria 1, 2 | P1, P2 |
| FR4 | The morning time is the start of `[time] business_hours`. | #939 criterion 3 | P1 |
| FR5 | A snooze of a question past its due time needs a reason, and a comment tells the seat. | #884 Scope; #939 criteria 4, 5; Decision #862 option 1 | P3 |
| FR6 | Ask someone: a mention of a person, or `aayu counsel --post`; the card moves to "With others" and comes back on the answer or at a time. | #884 Scope; #939 criteria 6, 7 | P4 |

## Non-functional requirements

| Kind | Requirement | Source |
|---|---|---|
| Consistency | Two machines agree, so the ledger record is the only state (inferred from #884 Summary). | #884 |
| Security | Only the ledger writer writes the ledger; agents never commit code to it. | AGENTS.md, "Work items" |
| Security | Issue text on a card stays terminal-safe (`termText`). | `InboxItem` comment (`internal/cli/inbox.go:247-248`) |
| Operability | New keys (for example default wake times) go to `internal/config` with a `CONSTRAINTS.md` row. | #884 Expected paths (`internal/config/**`, `CONSTRAINTS.md`); AGENTS.md "Generated files" |

## Open questions

- Which config keys are needed? #884 lists `internal/config/**` and `CONSTRAINTS.md` but names no key.
- "Ask someone" posts a mention in the person's name. Is that a write through `aayu` with the actions log? The sources
  do not say how.
