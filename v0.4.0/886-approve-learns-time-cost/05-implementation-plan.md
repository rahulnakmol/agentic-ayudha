# Implementation plan: Approve to build learns time and cost

## Stories and Bugs

Feature #886 has no Stories or Bugs (`_data/subs.tsv`). The rows below are proposed, not filed. They follow the #886 Scope
and the two hooks on `dev`. No source estimates effort or risk.

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| P1 | Working hours per effort size are learned from merged stories, with a default per size under a config key | Story (proposed, not filed) | not filed | not estimated | not estimated | #872 P1, P2 | `internal/delivery/**`, `internal/cli/missiontime.go` (`Env.workingHours`), `internal/config/**` | not started |
| P2 | Approve to build shows a Feature's cost estimate and range, learned from merged stories | Story (proposed, not filed) | not filed | not estimated | not estimated | #872 P1, P2 | `internal/delivery/**`, `internal/cli/mission.go` (`Env.costEstimate`) | not started |
| P3 | A mission's budget holds on each story's measured spend, not on the per-story cap | Story (proposed, not filed) | not filed | not estimated | not estimated | #872 P2 | `internal/cli/missionnext.go`, `internal/cli/missiongate.go`, `internal/cli/missionask.go`, `internal/cli/mission.go` (`missionUsed`) | not started |
| P4 | Slice, grill, and the guide name the estimate and its basis (inferred) | Story (proposed, not filed) | not filed | not estimated | not estimated | P1, P2 | `internal/planning/**`, `internal/grill/**`, `skills/slice/**`, `docs/guide/**` | not started |

The Expected paths also list `internal/workitems/**`, `internal/initrepo/**`, `internal/ready/**`, and
`internal/github/**`. The body does not say what changes there. Those paths are not given a slice here.

## Delivered

Nothing under #886. Hooks and types from Feature #889's Stories that #886 fills:

- `CostEstimate`, `Env.costEstimate`, and the budget basis text, in
  [d941d15](https://github.com/rahulnakmol/agentic-ayudha/commit/d941d151) (#1084, Story #997).
- `WorkingHours`, `MissionHours`, `Env.workingHours`, and the per-story effort, in
  [2c6c6b0](https://github.com/rahulnakmol/agentic-ayudha/commit/2c6c6b04) (#1091, Story #1015).

## In review

None (`_data/open-prs.tsv`).

## Pending

- P1 to P4.
- History: no `delivery/stories.jsonl` exists, so even after P1 and P2 the estimates fall back to defaults until
  #872 has written samples. #872 says "The data must start early so that v0.4.0 has samples" (#872 Problem).
- Person steps: #886's baselines and Approve to build (none on the ledger).
- The default-hours key and "#886 S8" (`internal/cli/missiontime.go:40-46`): unverified.
