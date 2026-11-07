# #883 Spec

No spec baseline exists (`specs/883` is absent from the ledger) and no Story is filed under #883. Requirements trace
to #883, Decision #862 option 1 (accepted), and Story #940's Completion Goal (under Feature #975). Slices are proposed,
not filed (see [05-implementation-plan.md](05-implementation-plan.md)).

## Functional requirements

| # | Requirement | Source | Slice |
|---|---|---|---|
| FR1 | From 0 to 24 h a question is in the Answer area; its age turns amber in the last 4 hours. | [#862](https://github.com/rahulnakmol/agentic-ayudha/issues/862) option 1; [#940](https://github.com/rahulnakmol/agentic-ayudha/issues/940) criterion 1 | P1, P4 |
| FR2 | From 24 to 48 h it moves to an Overdue area right after Now. A banner on every view counts the overdue questions and names the seats that wait. | #862 option 1; #940 criterion 2; #883 Scope | P4 |
| FR3 | The sweep pages once at 24 h. | #862 option 1; #883 Scope ("one page at 24 h") | P2 |
| FR4 | At 48 h a two-way question takes the seat's recommendation provisionally; it shows in Decide to ratify or revert, and the asking session is told. | #862 option 1; #940 criterion 4 | P3 |
| FR5 | A one-way question never applies on its own; it stays in Overdue until a person answers. | #862 option 1; #940 criterion 5 | P3 |
| FR6 | Both clocks use `[decisions] sla_hours` and `provisional_after_hours`, in business hours; the solo window `solo_provisional_after_hours = 24` still applies. | #862 option 1 and Evaluation; `internal/config/defaults.toml:199-203` | P1 |
| FR7 | A snooze past the due time needs a one-line reason, and the seat is told. | #862 option 1; #883 Scope | P4 (with #884 P3) |
| FR8 | The sweep delivers deferred pages. | #862 Evaluation ("It must also deliver deferred pages, which never happens today (Bug #859)") | P2 |

## Non-functional requirements

| Kind | Requirement | Source |
|---|---|---|
| Safety | A provisional answer is policy, labelled and delivered as provisional, and a person can revert it. | #862 Evaluation |
| Reliability | One sweep, `decideSweep`, handles Decisions and questions. | #862 Evaluation |
| Reliability | A page is sent once per key (today the keys are `decision-sla:<repo>#<n>` and `decision-overdue:<repo>#<n>`). | `internal/cli/decide.go:214-218` |
| Operability | `[decisions]` keys are config in `internal/config/`; a change needs a person. | `.aayudha/config.toml` `[autonomy] always_person` |

## Open questions

- Where do questions live? #880 ("aayu ask") is not sliced, so the record format is not in any source.
- Do mission asks (`MissionAskMarker`, `internal/cli/missionask.go`) count as questions for the SLA? The sources do
  not say.
