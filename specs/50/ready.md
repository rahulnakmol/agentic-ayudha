# rahulnakmol/agentic-ayudha#50: WP4.10 Thin-slice verbs

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `ac43c0699048ebb2bc0af5ec003fc4d1b6cad29d598227be2c0983d37509f661` |
| Completion Goal hash | `e49e6663ede209f721c1ab5001e2fafbb50b007e78bf7100c4b2eab55f0b9330` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/50/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the WP4.10 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: tier-1 eval for `grill story` yields an issue passing `ready`
2. [api-trace] state: the WP4.10 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `build` on a non-Ready issue exits 4
3. [api-trace] state: the WP4.10 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `ship` with only a bot review exits 4

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP4.10 Thin-slice verbs built as docs/BUILD-PLAN.md specifies, so that M4 (Proof) delivers: Every merge carries a CI-authoritative proof pack: pack format, ledger with LFS and media store, deterministic floors, merge gate and inspect, thin-slice verbs.

Plan text (docs/BUILD-PLAN.md, WP4.10): The minimum of three M5 verbs needed to run one story end to end, built here so M4.5 does not wait for M5: `grill story` as a **markdown-only skill** (question set from `references/story.md`, no TUI, no baseline adapters; writes the issue via `actions`); `build` minimal (refuse unless Ready and body hash = snapshot; worktree from `origin/dev`; checkpoint commits with the `Aayudha-Checkpoint` trailer; open PR; no lease, no budget breaker, no `--resume`); `ship` minimal (green gate, one approving human review, squash-merge, Complete section, cleanup). WP5.3/5.5 extend these in place; nothing here is thrown away.

### Completion Goal

1. [api-trace] state: the WP4.10 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: tier-1 eval for `grill story` yields an issue passing `ready`
2. [api-trace] state: the WP4.10 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `build` on a non-Ready issue exits 4
3. [api-trace] state: the WP4.10 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `ship` with only a bot review exits 4

### Measure

WP4.10 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP4.10)

### Threshold

all 3 criteria

### Scenario

Open the WP4.10 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
