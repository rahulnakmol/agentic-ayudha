# Spec: Approve to build learns time and cost

No spec baseline and no Story exist. Each requirement is traced to the #886 body or the code hooks that name #886,
and to a proposed slice in [05-implementation-plan.md](05-implementation-plan.md). Proposed slices are not filed.

## Functional requirements

| # | Requirement | Source | Proposed slice |
|---|---|---|---|
| FR1 | A time estimate learned from `delivery/stories.jsonl`, read by Approve to build and the mission clock. | #886 Scope | P1 |
| FR2 | The time estimate is working hours per effort size, learned where a size has history, else a default under a key #886 names. | `WorkingHours` comment, `internal/cli/missiontime.go:39-47` ("#886 S8") | P1 |
| FR3 | A cost estimate of a Feature, a point and a range in USD, with source `#886`. | `CostEstimate`, `internal/cli/mission.go:309-315` | P2 |
| FR4 | The budget holds on measured spend, not on the per-story cap. | #886 Scope; `internal/cli/missionnext.go:369` (`:359` at 884d81b), `internal/cli/mission.go:1398-1406` | P3 |
| FR5 | Slice, grill, and the guide show the estimate and its basis (inferred from #886 Expected paths `skills/slice/**`, `internal/grill/**`, `internal/planning/**`, `docs/guide/**`). | inferred | P4 |

## Non-functional requirements

| Area | Requirement | Source |
|---|---|---|
| Data | Each number names its source. Approve to build prints each number with its source today (`aayu mission show`: "print the plan, each number with its source", `internal/verbs/registry.go:873`). | code |
| Data | With no history, the estimate says so. Today: "with none, the plan has no estimate in hours, and nothing warns or refuses at Start" (`internal/cli/missiontime.go:49-51`). | code |
| Cost | The default budget is `[budget] per_story_usd = 60.0` and `per_day_usd = 100.0` (`internal/config/defaults.toml:114-116`). | code |
| Reliability | Decision #868 (cancelled) planned a 3 h default "with fewer than 5 samples". No accepted source sets a sample rule now. Unverified. | #868 |
