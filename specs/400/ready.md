# rahulnakmol/agentic-ayudha#400: Fail CI when a PR lowers a skill-eval bar without an accepted waiver

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `8febac9a5dcd3f98b1147874b06451d5e806abbdfbb0bc1c007edeaf12bb9f43` |
| Completion Goal hash | `9e5d7524e90d68152918edcc6bc20904f020751558da7abfc43aadf2d4443000` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/400/ready.md` |

Completion Goal at Ready:

1. [metric] state: a PR that lowers min_mean, min_pass_k, or pass_score in evals/eval.toml, sets a group to advisory, or deletes a rubric line from a case, without naming an accepted Decision; expected: `TestEvalBarRatchet` fails it, and passes once the PR names one

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

Replaces #331, which retro --classify filed from Feedback #327 without the fields Ready needs. As the founder, I want a pull request that lowers a skill-eval bar to fail CI unless it names a floor waiver a human accepted, so that an agent cannot make a failing evaluation pass by changing the evaluation (Feedback #327: 13 times in 9 threads, each caught only by a reviewer). The bar ratchets as the coverage floor in `ratchets.toml` does.

### Completion Goal

1. [metric] state: a PR that lowers min_mean, min_pass_k, or pass_score in evals/eval.toml, sets a group to advisory, or deletes a rubric line from a case, without naming an accepted Decision; expected: `TestEvalBarRatchet` fails it, and passes once the PR names one

### Measure

Completion Goal criteria observed

### Threshold

all 1 criteria

### Scenario

From a base of this repository's evals, change the head to lower each bar in turn (min_mean, min_pass_k, pass_score, a group set to advisory, a rubric line deleted) and run the ratchet check with no waiver named, then with a new accepted eval-bar waiver Decision named in evals/eval.toml.

### Environment

local checkout and this repository's CI build job on a pull request; synthetic eval fixtures, never production

### Checkpoints

1. `go test ./internal/evals -run TestEvalBarRatchet` shows each kind of lowering failing with no waiver and passing with a new accepted one (criteria 1)
2. a pull request that lowers min_mean fails the CI build job's eval-bar ratchet step, naming the bar and the `aayu decide --waiver eval-bar` command (criteria 1)

### Assumptions

- a waiver is the existing floor-waiver Decision (`aayu decide --waiver eval-bar <story>`, accepted by a human with `aayu decide <n> --accept waive`), named in evals/eval.toml as `bar_waiver = <n>`, as ratchets.toml names a coverage waiver
- a reworded rubric line counts as deleted when it drops any of the line's words; a line that only gains words does not

### Provenance

Request: "build them!"
1. confirmed (request): "build them!"
````
