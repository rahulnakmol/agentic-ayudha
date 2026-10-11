# rahulnakmol/agentic-ayudha#58: WP6.4 Figma + Mobbin

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `28bfd9d724a00618442ffd64005c0b49bb1be04488a55bbbf410d48516372d71` |
| Completion Goal hash | `02b0818a3d4990c66e7543fa56708f9b09f3f3b02ac457cb5822953a82581861` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/58/ready.md` |

Completion Goal at Ready:

1. [screenshot] state: the WP6.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `doctor` reachability
2. [screenshot] state: the WP6.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `research` fixture returns a reference set with URLs only

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP6.4 Figma + Mobbin built as docs/BUILD-PLAN.md specifies, so that M6 (UX) delivers: Figma and Mobbin references, research to prototype, review and polish, experiments and readouts.

Plan text (docs/BUILD-PLAN.md, WP6.4): Figma REST + PAT primary (`get images`, nodes, variables), remote MCP opt-in; Mobbin MCP client returning **URLs and IDs only, never pixels** (terms verified by judge 3); lint blocks Mobbin-origin images anywhere.

### Completion Goal

1. [screenshot] state: the WP6.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `doctor` reachability
2. [screenshot] state: the WP6.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `research` fixture returns a reference set with URLs only

### Measure

WP6.4 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP6.4)

### Threshold

all 2 criteria

### Scenario

Open the WP6.4 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
