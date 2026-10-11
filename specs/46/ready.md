# rahulnakmol/agentic-ayudha#46: WP4.6 Floors: DAST + API

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `5fcc1920886b5e040b0dfbc4513d55340e2a4686937b4ae1a3ebea8c65c6b41f` |
| Completion Goal hash | `efaad0eface3121229579f350678415895a222db99a53ee72e7598d1fae987e5` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/46/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the WP4.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: fixture with a reflected-XSS route fails

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP4.6 Floors: DAST + API built as docs/BUILD-PLAN.md specifies, so that M4 (Proof) delivers: Every merge carries a CI-authoritative proof pack: pack format, ledger with LFS and media store, deterministic floors, merge gate and inspect, thin-slice verbs.

Plan text (docs/BUILD-PLAN.md, WP4.6): ZAP baseline against the running app when `[verify] dast = true`; k6 thresholds for API/CLI contracts.

### Completion Goal

1. [api-trace] state: the WP4.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: fixture with a reflected-XSS route fails

### Measure

WP4.6 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP4.6)

### Threshold

all 1 criteria

### Scenario

Open the WP4.6 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
