# rahulnakmol/agentic-ayudha#63: WP7.2 `measure`

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `f64128b3c1a6a3a40168b03d6bea01377a5ddaf50eff87a15abd57f7850be278` |
| Completion Goal hash | `6fc9f9fc4407824ab8c39d7cbfb221af079fb3fe26272d86daea38a1e104a7d7` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/63/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: fixture exports; expected: expected cohorts

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP7.2 `measure` built as docs/BUILD-PLAN.md specifies, so that M7 (Loops and explain) delivers: Scheduled loops, measure, brief, tell, audit and retro, evolve, release.

Plan text (docs/BUILD-PLAN.md, WP7.2): Extends WP5.2 adapters with GrowthBook, billing CSV, App Insights/AWS/GCP exports; SQLite (`modernc.org/sqlite`) snapshots, DuckDB optional shell-out; AARRR/North Star queries; Results on Goal/Initiative; triggers parent-close rule.

### Completion Goal

1. [api-trace] state: fixture exports; expected: expected cohorts

### Measure

WP7.2 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP7.2)

### Threshold

all 1 criteria

### Scenario

Open the WP7.2 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
