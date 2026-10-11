# rahulnakmol/agentic-ayudha#67: WP7.6 `audit`, `retro`

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `e0d23d20c8fcf0f8634cce2fe464b9ae349c5effefeba474c6f4f56b66e8b0b8` |
| Completion Goal hash | `7aafed3ca2314a9874af4c886eef7b67b49c3a134775da273434fdd9c455d22a` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/67/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the WP7.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: dogfood outputs match fixtures
2. [api-trace] state: the WP7.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a direct `gh` write outside `actions` is flagged
3. [api-trace] state: the WP7.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a solo-mode self-approval (WP0.1) and a human-fallback write (WP1.4) each appear in the audit report
4. [api-trace] state: the WP7.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: re-running `audit` with the same recorded inputs produces a byte-identical report

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP7.6 `audit`, `retro` built as docs/BUILD-PLAN.md specifies, so that M7 (Loops and explain) delivers: Scheduled loops, measure, brief, tell, audit and retro, evolve, release.

Plan text (docs/BUILD-PLAN.md, WP7.6): `audit`: floors, proof coverage, waivers, decision SLA, autonomy exceptions, reconciliation of GitHub audit events vs `actions` segments (flags unlogged writes), license scan; report to `ledger/audit/<date>/`. `retro`: estimate vs observed checkpoints, correction rate, streaks, floor misses, cost per story; files findings as **Feedback issues on `agentic-ayudha`**, never PRs. Both record their inputs (`ledger` SHA, GitHub event cursor, actions segment list, config hash) in the report header so a report can be regenerated and diffed.

### Completion Goal

1. [api-trace] state: the WP7.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: dogfood outputs match fixtures
2. [api-trace] state: the WP7.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a direct `gh` write outside `actions` is flagged
3. [api-trace] state: the WP7.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a solo-mode self-approval (WP0.1) and a human-fallback write (WP1.4) each appear in the audit report
4. [api-trace] state: the WP7.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: re-running `audit` with the same recorded inputs produces a byte-identical report

### Measure

WP7.6 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP7.6)

### Threshold

all 4 criteria

### Scenario

Open the WP7.6 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
