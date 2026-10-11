# rahulnakmol/agentic-ayudha#43: WP4.3 Floors framework + coverage

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `a4872096e3f8d01b55b3273b2c9144d4cd41df384eef881348983838d0505fcb` |
| Completion Goal hash | `100c888f2bc8cb3d78f5d464fe249a0b396e5735764ae93bdb65d329f7ef1c42` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/43/ready.md` |

Completion Goal at Ready:

1. [metric] state: the WP4.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: 89.9 % fails, 90.0 % passes
2. [api-trace] state: the WP4.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: child process cannot read `AAYU_*` or `GITHUB_TOKEN` (test)

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP4.3 Floors framework + coverage built as docs/BUILD-PLAN.md specifies, so that M4 (Proof) delivers: Every merge carries a CI-authoritative proof pack: pack format, ledger with LFS and media store, deterministic floors, merge gate and inspect, thin-slice verbs.

Plan text (docs/BUILD-PLAN.md, WP4.3): `internal/verify` runner: floor interface, scrubbed child env, no-network default, results into pack; diff coverage from lcov, cobertura, and Go coverprofile; threshold from config (default 90 %).

### Completion Goal

1. [metric] state: the WP4.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: 89.9 % fails, 90.0 % passes
2. [api-trace] state: the WP4.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: child process cannot read `AAYU_*` or `GITHUB_TOKEN` (test)

### Measure

WP4.3 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP4.3)

### Threshold

all 2 criteria

### Scenario

Open the WP4.3 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
