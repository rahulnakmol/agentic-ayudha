# rahulnakmol/agentic-ayudha#54: WP5.4 `slice`, `plan`

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `45956e43fb7f7d31dfa72a8644d396a1bff1de349905663ee8e431132f58c08a` |
| Completion Goal hash | `d6e6d9385ca60e93371e06915fad5b0252449fccc7b0bba445fa3d8441874cc9` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/54/ready.md` |

Completion Goal at Ready:

1. [metric] state: the WP5.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: evals on two fixture features (one 3-layer, one 6-layer incl. L07 migration): every produced story is ≤ L and passes `ready` (tier 1, 100 %)
2. [api-trace] state: the WP5.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: each `plan` has exactly one checkpoint per layer in the story's footprint and a rollback step for any L07 checkpoint
3. [api-trace] state: the WP5.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: tier-2 rubric meets the WP3.4 bar
4. [api-trace] state: the WP5.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: an XL-sized input feature is returned as ≥ 2 stories with the XL closed "split into"

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP5.4 `slice`, `plan` built as docs/BUILD-PLAN.md specifies, so that M5 (Seats) delivers: The twelve seats with charters and RACI, grill for every type, slice and plan, build and ship, decide and respond, autonomy and halt.

Plan text (docs/BUILD-PLAN.md, WP5.4): `slice`: XS–L stories with criteria, deps, footprint **computed from the Feature's expected paths via `LAYERS.md` globs and confirmed by the plan**; closes an XL as "split into". `plan`: checkpoints per layer, contracts, tests, instrumentation, rollback; EM plan-time review required for `autonomy.never` layers, recorded in the pack.

### Completion Goal

1. [metric] state: the WP5.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: evals on two fixture features (one 3-layer, one 6-layer incl. L07 migration): every produced story is ≤ L and passes `ready` (tier 1, 100 %)
2. [api-trace] state: the WP5.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: each `plan` has exactly one checkpoint per layer in the story's footprint and a rollback step for any L07 checkpoint
3. [api-trace] state: the WP5.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: tier-2 rubric meets the WP3.4 bar
4. [api-trace] state: the WP5.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: an XL-sized input feature is returned as ≥ 2 stories with the XL closed "split into"

### Measure

WP5.4 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP5.4)

### Threshold

all 4 criteria

### Scenario

Open the WP5.4 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
