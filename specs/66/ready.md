# rahulnakmol/agentic-ayudha#66: WP7.5 `tell`

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `a3c88ea13a8a1923b017469db403d82b446f7f6fc23b4370e0fb8d64bb6028ea` |
| Completion Goal hash | `c596e0a96f1d1cc6a3a015fbf34f4986b95d80aac27947064526598b9259e978` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/66/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the WP7.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: guide references only proven Features
2. [api-trace] state: the WP7.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: publish refuses bot identity

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP7.5 `tell` built as docs/BUILD-PLAN.md specifies, so that M7 (Loops and explain) delivers: Scheduled loops, measure, brief, tell, audit and retro, evolve, release.

Plan text (docs/BUILD-PLAN.md, WP7.5): `docs|guide|changelog|release|launch|brand`; tone from `brand.yaml`; claims link only to Features with authoritative packs; UTM tagging; `tell publish` human-identity only, logged.

### Completion Goal

1. [api-trace] state: the WP7.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: guide references only proven Features
2. [api-trace] state: the WP7.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: publish refuses bot identity

### Measure

WP7.5 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP7.5)

### Threshold

all 2 criteria

### Scenario

Open the WP7.5 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
