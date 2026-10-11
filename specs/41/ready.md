# rahulnakmol/agentic-ayudha#41: WP4.1 Pack format

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `9604fb1a094d54b8c02c1e11e45e2c08d2d3b87c522f3950f40e57d240b65804` |
| Completion Goal hash | `9a10141be80233ba2b52aca947a458defb7d33796f9e45fc6ab6ad6d0a1dd0e9` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/41/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the WP4.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: schema validates samples
2. [api-trace] state: the WP4.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: hash mismatch, missing criterion mapping, and `sha ≠ head` all detected by `proof verify`

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP4.1 Pack format built as docs/BUILD-PLAN.md specifies, so that M4 (Proof) delivers: Every merge carries a CI-authoritative proof pack: pack format, ledger with LFS and media store, deterministic floors, merge gate and inspect, thin-slice verbs.

Plan text (docs/BUILD-PLAN.md, WP4.1): `pack.json` v1: story, **pr_head_sha**, checkpoint, seat, host, model, identity, `completion_goal_hash`, `criteria[] {id, evidence: [artifact refs]}`, artifacts[] {path (always `/`), kind, sha256, size, lfs|bucket}, checks[] {floor, result, value, threshold, tool, version, waiver?}, cost fields, links. Schema versioned.

### Completion Goal

1. [api-trace] state: the WP4.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: schema validates samples
2. [api-trace] state: the WP4.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: hash mismatch, missing criterion mapping, and `sha ≠ head` all detected by `proof verify`

### Measure

WP4.1 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP4.1)

### Threshold

all 2 criteria

### Scenario

Open the WP4.1 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
