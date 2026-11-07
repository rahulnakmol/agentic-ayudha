# Spec: one benchmark line per merged story

No spec baseline and no Story exist. Each requirement is traced to the #872 body and to a proposed slice (P1 to P4)
in [05-implementation-plan.md](05-implementation-plan.md). Proposed slices are not filed.

## Functional requirements

| # | Requirement | Source | Proposed slice |
|---|---|---|---|
| FR1 | A story line has: drafted, Ready, first build start, PR, first review, merged, and released times. | #872 Scope, In | P1 (schema), P2 (written) |
| FR2 | A story line has: rounds, measured cost, host, model, the estimate and its basis, and the origin (loop, Build now, or a thread). | #872 Scope, In | P1, P2 |
| FR3 | `ship` appends one line to `delivery/stories.jsonl` at merge. | #872 Summary | P2 |
| FR4 | `release` appends one line per release to `delivery/releases.jsonl`. | #872 Summary | P3 |
| FR5 | The released time reaches the story's record. Inferred: the story line is written at merge, before the release, so release adds it (in its own line or a later line). The source does not say how. | inferred from #872 Scope | P3 |
| FR6 | `audit` and `retro` read the records (inferred from #872 Expected paths `internal/cli/audit.go`, `internal/cli/retro.go`, `internal/audit/**`). | inferred | P4 |

## Non-functional requirements

| Area | Requirement | Source |
|---|---|---|
| Reliability | One line per story: a retried `ship` must not add a second line. Inferred from "one line per story" (#872 Summary). | inferred |
| Data | The host and model come from #871's source (#872 Links: depends-on #871). | #872 |
| Cost | "Measured cost" is the spend aayu measures. Today that is the local actions log (`Env.costs`, `internal/cli/status.go:97`) plus `AAYU_COST_USD` (`Env.buildCost`, `internal/cli/build.go:1142`). Inferred: a cloud receiver's spend may not be in it. | code; inferred |
| Operability | The ledger branch takes data only (AGENTS.md, "Work items": "Never commit code to it or merge it"). | AGENTS.md |
