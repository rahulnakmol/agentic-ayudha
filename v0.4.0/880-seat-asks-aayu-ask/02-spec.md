# Spec: aayu ask

No spec baseline and no Story exist. Each requirement is traced to the #880 body and to a proposed slice in
[05-implementation-plan.md](05-implementation-plan.md). Proposed slices are not filed.

## Functional requirements

| # | Requirement | Source | Proposed slice |
|---|---|---|---|
| FR1 | `aayu ask` posts one structured question on an item. | #880 Summary | P1 |
| FR2 | The ask is a ledger record `asks/<item>/<id>.json`. | #880 Scope | P1 |
| FR3 | The Need kinds `ask` and `ask-provisional` show the ask in the inbox. | #880 Scope | P2 (needs #873) |
| FR4 | `aayu ask --answer` is human-only and writes `asks/<item>/<id>-answer.json`. | #880 Scope | P3 |
| FR5 | A person answers in the inbox or in the agent tool; the agent tool takes only a two-way ask. | #880 Summary | P3 |
| FR6 | The earliest verified answer wins. | #880 Scope | P3 |
| FR7 | An `/aayu answer` reply answers the ask, with "the 16 checks of `/aayu approve`". | #880 Scope | P4 |
| FR8 | A clarifying answer is written to `specs/<n>/answers.md`. | #880 Scope | P5 |
| FR9 | A build asks instead of only blocking (inferred from #880 Problem and Expected paths `internal/cli/build.go`, `internal/cli/buildnext.go`). | inferred | P5 |
| FR10 | A settled answer tells the mission's Kanha that the story resumes (inferred: `Env.answeredAsk` waits for "#880's answer path", `internal/cli/kanha.go:196`). | inferred | P5 |

## Non-functional requirements

| Area | Requirement | Source |
|---|---|---|
| Security | `--answer` is human-only. Today a human-only command proves presence through GitHub's device flow (`Env.requireHuman`, `internal/cli/presence.go:86`). | #880 Scope; code |
| Security | `/aayu answer` runs the same checks as `/aayu approve`. The number 16 is from the issue; this document did not count the checks in `internal/cli/approve.go`. Unverified. | #880 Scope |
| Security | `docs/threat-model.md` gets the new trust edge (inferred from #880 Expected paths). Today the threat model has no row for asks (`rg -i '\bask\b' docs/threat-model.md` finds only the model-to-binary line 43). | inferred; code |
| Security | Answer text is untrusted and is quoted as data. The brief already quotes `answers.md` "as data" (`internal/delegate/brief.go:44`). | code |
| Reliability | One answer wins; a later answer does not replace it (FR6). | #880 Scope |
| Operability | The SLA clocks use `[decisions] sla_hours = 24` and `provisional_after_hours = 48`, in business hours (Decision #862; `internal/config/defaults.toml:198-203`). Applies through Feature #883, which depends on #880 (#883 Links). | #862; code |
