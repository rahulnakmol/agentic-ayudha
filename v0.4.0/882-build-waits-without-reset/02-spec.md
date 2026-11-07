# Spec: a build waits on a person without a reset

No spec baseline and no Story exist. Each requirement is traced to the #882 body and to a proposed slice in
[05-implementation-plan.md](05-implementation-plan.md). Proposed slices are not filed.

## Functional requirements

| # | Requirement | Source | Proposed slice |
|---|---|---|---|
| FR1 | The build state has `waiting` and `paused`. | #882 Scope | P1 |
| FR2 | The wall clock pauses until the answer comes. | #882 Summary | P1 |
| FR3 | A build that asks keeps its budget, and a clarifying answer causes no reset. | #882 Summary and Scope | P1 |
| FR4 | The lease extends to the provisional time plus 4 h. | #882 Scope | P2 |
| FR5 | The stale sweep skips a waiting build. | #882 Scope | P2 |
| FR6 | "Change the story" keeps today's reset. | #882 Scope, Out | P1 (a guard that the reset path stays) |
| FR7 | The config and `CONSTRAINTS.md` name the new limit (inferred from #882 Expected paths `internal/config/config.go`, `defaults.toml`, `gen/main.go`, `CONSTRAINTS.md`). | inferred | P3 |

"The provisional time" is the time at which a two-way ask applies the seat's recommendation provisionally: 48
business hours, `[decisions] provisional_after_hours` (Decision #862; `internal/config/defaults.toml:200`). In solo
mode it is 24 h (`solo_provisional_after_hours`, `:203`; #862 Evaluation). This link is inferred; #882 does not cite #862.

## Non-functional requirements

| Area | Requirement | Source |
|---|---|---|
| Reliability | Another builder is still refused while the extended lease lives (today's lease rule, `Env.takeLease`, `internal/cli/lease.go:101-108`). Inferred. | code; inferred |
| Cost | A paused build spends no wall-clock budget. Spend in USD is not paused; the evidence does not say it should be. | #882 Summary |
| Operability | `aayu gc` and the build-ready loop's stale claim check (`StaleGrace = LeaseTTL`, `internal/cli/buildnext.go:367-372`) agree on a waiting build. Inferred from "the stale sweep skipping a waiting build". | #882 Scope; code |
