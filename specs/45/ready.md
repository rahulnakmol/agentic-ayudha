# rahulnakmol/agentic-ayudha#45: WP4.5 Floors: web a11y + perf

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `9fc3529d5bd71d6c2e2473d584e97d65fa461a483cd2ce9464d839761c381926` |
| Completion Goal hash | `17ed4dab7205c6f4146b6560a2296de98990cf8c944b6502b618a38bdc5b7d47` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/45/ready.md` |

Completion Goal at Ready:

1. [screenshot] state: the WP4.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: planted contrast defect fails
2. [screenshot] state: the WP4.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: planted email in a screenshot blocks
3. [screenshot] state: the WP4.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: capture against a URL whose config marks it production exits 4 with no artifacts written

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP4.5 Floors: web a11y + perf built as docs/BUILD-PLAN.md specifies, so that M4 (Proof) delivers: Every merge carries a CI-authoritative proof pack: pack format, ledger with LFS and media store, deterministic floors, merge gate and inspect, thin-slice verbs.

Plan text (docs/BUILD-PLAN.md, WP4.5): Playwright drives every state listed in the Completion Goal; axe (zero serious/critical, WCAG 2.2 AA); keyboard-only pass; reduced-motion; 200 % text scaling; 44×44 targets; Lighthouse CI budgets from config; **seed data**: `verify` runs the app against `[verify] seed` (a fixture command or SQL that the product repo provides; `init` scaffolds `.aayudha/seed/` with a README) and refuses to capture when `AAYU_ENV` resolves to production; **PII redaction pass** (DOM masking + OCR/regex scan) that blocks the pack.

### Completion Goal

1. [screenshot] state: the WP4.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: planted contrast defect fails
2. [screenshot] state: the WP4.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: planted email in a screenshot blocks
3. [screenshot] state: the WP4.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: capture against a URL whose config marks it production exits 4 with no artifacts written

### Measure

WP4.5 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP4.5)

### Threshold

all 3 criteria

### Scenario

Open the WP4.5 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
