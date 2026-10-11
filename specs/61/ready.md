# rahulnakmol/agentic-ayudha#61: WP6.7 `experiment`, `readout`

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `ea9d8067d0e7d3c186cbaddd7c8cee6de197f6bc78ad039e58ad045de3241cd5` |
| Completion Goal hash | `f1252ce796803e75d93262c745c3da95636e9acc8026b7432f025c6201288da6` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/61/ready.md` |

Completion Goal at Ready:

1. [metric] state: the WP6.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: fixture lift computed
2. [api-trace] state: the WP6.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: under-sample refused

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP6.7 `experiment`, `readout` built as docs/BUILD-PLAN.md specifies, so that M6 (UX) delivers: Figma and Mobbin references, research to prototype, review and polish, experiments and readouts.

Plan text (docs/BUILD-PLAN.md, WP6.7): GrowthBook + PostHog; minimum sample size; writes Results.

### Completion Goal

1. [metric] state: the WP6.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: fixture lift computed
2. [api-trace] state: the WP6.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: under-sample refused

### Measure

WP6.7 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP6.7)

### Threshold

all 2 criteria

### Scenario

Open the WP6.7 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
