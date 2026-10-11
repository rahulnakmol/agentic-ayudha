# rahulnakmol/agentic-ayudha#69: WP7.8 `release`

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `e035e177ce2ed15c15951f1bbb09194beaf328825a033d6b733d866fb0dc0e90` |
| Completion Goal hash | `deb9aeab7f0139720513080f8925332afafa648a78cb56d060438df3a3572fdf` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/69/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the WP7.8 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: dogfood rolling snapshot with stamped stories

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP7.8 `release` built as docs/BUILD-PLAN.md specifies, so that M7 (Loops and explain) delivers: Scheduled loops, measure, brief, tell, audit and retro, evolve, release.

Plan text (docs/BUILD-PLAN.md, WP7.8): Promotion ladder for a Release issue; artifact identity; stamps included stories `released`; `tell release`; shipping evidence; triggers `measure` schedule.

### Completion Goal

1. [api-trace] state: the WP7.8 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: dogfood rolling snapshot with stamped stories

### Measure

WP7.8 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP7.8)

### Threshold

all 1 criteria

### Scenario

Open the WP7.8 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
