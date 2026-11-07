# Spec: you write behaviours

No spec baseline and no Story exist. Each requirement is traced to the #891 body or to Decision #867 option 1 (the
accepted option), and to a proposed slice in [05-implementation-plan.md](05-implementation-plan.md). Proposed slices
are not filed.

## Functional requirements

| # | Requirement | Source | Proposed slice |
|---|---|---|---|
| FR1 | Each map check gets `says` (the founder's sentence), `rule` (capability, order, retry, authority, never, or limit), and `source` (the Decision, story criterion, grill session, or spec baseline that set it). The name `kind` stays for `ui`, `cli`, `api`, `lib`. | #891 Scope; #867 option 1 | P1 |
| FR2 | A check's status is derived: enforced, proposed, failing, stale, or never verified. Every failing check blocks, approved or not. | #867 option 1 | P1 |
| FR3 | An approval is one line in `behaviours/approvals.jsonl` on the ledger: the check ref, the hash of `says` and `rule`, the hash of the steps, the source, who, when, and how. | #867 option 1; #891 Scope | P2 |
| FR4 | The founder answers with `aayu decide --behaviour <ref> --pr <n> --approve` or `--refuse`, after the presence check. The approval pins the PR head. | #867 option 1; #891 Scope | P2 |
| FR5 | `inspect` holds a PR that changes the sentence, the rule, or a step of an approved check, or removes it. A BEHAVIOUR CHANGE card shows in Decide. A new check does not hold a PR. | #867 option 1; #891 Scope | P3 |
| FR6 | `build` stops when its diff changes an approved check; the builder asks instead. | #867 option 1 | P4 |
| FR7 | `aayu decide --accept` needs `--sets <ref>`, `--sets new:"<sentence>"`, or `--sets none`. | #867 option 1; #891 Scope | P5 |
| FR8 | A story criterion that ends with `; standing` must get a map check with `source` `#n/k` before `ship` merges it. | #867 option 1; #891 Scope ("standing criteria") | P6 |
| FR9 | This repository gets its own map first, starting with the inbox behaviours. | #867 option 1; #891 Scope; #891 Expected paths `.agents/skills/verify-aayudha/` | P7 |
| FR10 | ETHOS.md gets principle 12. `internal/verify/featuremap.go`, `internal/verify/behaviour.go`, and the approval reader join `[autonomy] always_person`. | #867 option 1; #891 Scope | P8 |
| FR11 | The Behaviours view data. | #891 Scope | P9. The view's fields are not specified. Unverified. |

## Non-functional requirements

| Area | Requirement | Source |
|---|---|---|
| Security | Only the founder approves a sentence, through the presence check; the approval pins the PR head. | #867 option 1 |
| Security | The gate code is in the narrow set, so the PR that builds it needs a person to ship. | #891 Scope; #867 Evaluation |
| Reliability | Checks run with no model (`verify` is `ModeNone`, #867 Evaluation). Any failed check fails the floor (`internal/verify/behaviour.go`, package comment). | #867; code |
| Operability | One card per PR, with the step diff, limits the number of cards on a rename (#867 Evaluation, Risk line). | #867 |
| Data | The `source` field keeps the criterion link after merge; the evidence map links a criterion to checks only while its story is verified (`internal/verify/evidence.go:43-56`, #867 Evaluation). | #867; code |
