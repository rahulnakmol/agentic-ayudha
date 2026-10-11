# rahulnakmol/agentic-ayudha#70: WP8.1 Dogfood

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `e83abcff59bd03d28bf54e7a09057eef0380dd9d987787bcb0ca304dfd1ac85e` |
| Completion Goal hash | `e437e526a2273eea64c43b46b03a469c0ab23af06553425788a2016f894e7aff` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/70/ready.md` |

Completion Goal at Ready:

1. [metric] state: the WP8.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `status` shows proof coverage 100 %
2. [api-trace] state: the WP8.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `brief close` exits 0, HTML has zero serious/critical axe findings, grounding footer lists N packs

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP8.1 Dogfood built as docs/BUILD-PLAN.md specifies, so that M8 (0.1) delivers: Full dogfood with 100 % proof coverage, docs site, rolling snapshot, installers.

Plan text (docs/BUILD-PLAN.md, WP8.1): `examples/benefits-qa/` (nine-layer example) built by the pod from `grill goal` to `release`; every story with an authoritative pack.

### Completion Goal

1. [metric] state: the WP8.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `status` shows proof coverage 100 %
2. [api-trace] state: the WP8.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `brief close` exits 0, HTML has zero serious/critical axe findings, grounding footer lists N packs

### Measure

WP8.1 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP8.1)

### Threshold

all 2 criteria

### Scenario

Open the WP8.1 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
