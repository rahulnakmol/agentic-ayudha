# rahulnakmol/agentic-ayudha#44: WP4.4 Floors: SAST, secrets, deps

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `fc314a42f313652b268f0d4560aa437f3437831dbd4ed8dc5b6b3c5359bb8591` |
| Completion Goal hash | `b1e06c6e6b44b38722e4ce071a53c8a4e878c94ab67c95d648273e0e5039f5e9` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/44/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the WP4.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: planted findings fail
2. [api-trace] state: the WP4.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: clean fixture passes

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP4.4 Floors: SAST, secrets, deps built as docs/BUILD-PLAN.md specifies, so that M4 (Proof) delivers: Every merge carries a CI-authoritative proof pack: pack format, ledger with LFS and media store, deterministic floors, merge gate and inspect, thin-slice verbs.

Plan text (docs/BUILD-PLAN.md, WP4.4): semgrep rulesets per language, gitleaks, osv-scanner; zero critical/high.

### Completion Goal

1. [api-trace] state: the WP4.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: planted findings fail
2. [api-trace] state: the WP4.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: clean fixture passes

### Measure

WP4.4 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP4.4)

### Threshold

all 2 criteria

### Scenario

Open the WP4.4 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
