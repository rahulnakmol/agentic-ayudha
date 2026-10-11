# rahulnakmol/agentic-ayudha#62: WP7.1 Loops

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `f1f7d9b7dd1c5c5f1a8f978d0bec20fff1f13d1ac63f4b02008fb2025de382fa` |
| Completion Goal hash | `a83cf91c9a89fa53eeef679fc4fc2b44b395c9e91868f8a2014a456596fe7a26` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/62/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the WP7.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: golden outputs per host (Amp schedule JSON, Cursor automation file, Actions cron with the tz-converted cron expression)
2. [api-trace] state: the WP7.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: the Actions fallback runs `status --digest` on the dogfood repo on schedule and the fake notify sink receives one digest
3. [api-trace] state: the WP7.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: setting `halt` makes the next scheduled run exit 4 with no writes

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP7.1 Loops built as docs/BUILD-PLAN.md specifies, so that M7 (Loops and explain) delivers: Scheduled loops, measure, brief, tell, audit and retro, evolve, release.

Plan text (docs/BUILD-PLAN.md, WP7.1): `loops/*.toml` → Amp schedules/plugin, Cursor Automations, Actions fallback; nightly `status --digest`, weekly `retro`, monthly seat self-review, quarterly `evolve`; all honour `halt` and `[budget]`; schedules are expressed in `[time] tz`.

### Completion Goal

1. [api-trace] state: the WP7.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: golden outputs per host (Amp schedule JSON, Cursor automation file, Actions cron with the tz-converted cron expression)
2. [api-trace] state: the WP7.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: the Actions fallback runs `status --digest` on the dogfood repo on schedule and the fake notify sink receives one digest
3. [api-trace] state: the WP7.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: setting `halt` makes the next scheduled run exit 4 with no writes

### Measure

WP7.1 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP7.1)

### Threshold

all 3 criteria

### Scenario

Open the WP7.1 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
