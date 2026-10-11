# rahulnakmol/agentic-ayudha#49: WP4.9 Floors: mobile

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `a1542bf1b23cce8d386d159853a54b55a32e2564d88917d40f021e30a6171a35` |
| Completion Goal hash | `2d0e84e3eaa21d8e8f1ac6e537044d12e85b8e8f834b138cb828ff4b84406612` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/49/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the WP4.9 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a story whose footprint includes `platforms: ios` produces a pack with `skipped: mobile`
2. [api-trace] state: the WP4.9 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `ship` on it exits 4
3. [api-trace] state: the WP4.9 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `ship` with a linked floor-waiver Decision proceeds and the pack records the waiver ID

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP4.9 Floors: mobile built as docs/BUILD-PLAN.md specifies, so that M4 (Proof) delivers: Every merge carries a CI-authoritative proof pack: pack format, ledger with LFS and media store, deterministic floors, merge gate and inspect, thin-slice verbs.

Plan text (docs/BUILD-PLAN.md, WP4.9): iOS XCUITest + `performAccessibilityAudit`; Android Espresso/Compose semantics on KVM runner. Until then a mobile story's pack carries `skipped: mobile` which **blocks `ship`** unless a Decision waiver exists. The 0.1 deliverable of this WP is only the skip-and-block behaviour.

### Completion Goal

1. [api-trace] state: the WP4.9 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a story whose footprint includes `platforms: ios` produces a pack with `skipped: mobile`
2. [api-trace] state: the WP4.9 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `ship` on it exits 4
3. [api-trace] state: the WP4.9 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `ship` with a linked floor-waiver Decision proceeds and the pack records the waiver ID

### Measure

WP4.9 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP4.9)

### Threshold

all 3 criteria

### Scenario

Open the WP4.9 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
