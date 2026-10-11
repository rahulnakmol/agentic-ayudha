# rahulnakmol/agentic-ayudha#52: WP5.2 Baseline adapters

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `d051eca9da6517e035c914c46885986af277ee5ed8c08d9bc0d74564e1b03bb4` |
| Completion Goal hash | `91ebca04b21941bd70ba811ef3cce3d335bc0f2dd8c908a54d82920e1ffb2c18` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/52/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: recorded PostHog/Sentry/GitHub fixtures; expected: the exact expected baseline tuples (golden)
2. [api-trace] state: the WP5.2 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a missing adapter yields exit 5 with the message naming the `[integrations]` key to set
3. [api-trace] state: the WP5.2 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a write-scoped key is refused by WP1.3 before any call

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP5.2 Baseline adapters built as docs/BUILD-PLAN.md specifies, so that M5 (Seats) delivers: The twelve seats with charters and RACI, grill for every type, slice and plan, build and ship, decide and respond, autonomy and halt.

Plan text (docs/BUILD-PLAN.md, WP5.2): PostHog, Sentry, GitHub read adapters (moved up from M7) so `grill` can pull baselines; each adapter returns `{metric, population, window, value, source_query, fetched_at}` so a baseline is reproducible.

### Completion Goal

1. [api-trace] state: recorded PostHog/Sentry/GitHub fixtures; expected: the exact expected baseline tuples (golden)
2. [api-trace] state: the WP5.2 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a missing adapter yields exit 5 with the message naming the `[integrations]` key to set
3. [api-trace] state: the WP5.2 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a write-scoped key is refused by WP1.3 before any call

### Measure

WP5.2 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP5.2)

### Threshold

all 3 criteria

### Scenario

Open the WP5.2 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
