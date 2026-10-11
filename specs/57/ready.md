# rahulnakmol/agentic-ayudha#57: WP5.7 Autonomy, notify, halt

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `9297204049ccac100cd49b0edf2a37ff6803385d3ce55a92530b9a62c189fc91` |
| Completion Goal hash | `1cd54620aa1262639363f18fc60991dccdd0b554a6608408e5b63b6674d799a4` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/57/ready.md` |

Completion Goal at Ready:

1. [metric] state: the WP5.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: streak math at 19/20/21 and on each correction kind
2. [api-trace] state: the WP5.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `halt` stops a running loop within one poll

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP5.7 Autonomy, notify, halt built as docs/BUILD-PLAN.md specifies, so that M5 (Seats) delivers: The twelve seats with charters and RACI, grill for every type, slice and plan, build and ship, decide and respond, autonomy and halt.

Plan text (docs/BUILD-PLAN.md, WP5.7): `aayudha-streak.yml` (protected) computes streak per layer footprint from merged-PR history; corrections detected automatically: changes-requested review, non-bot commit on the PR, post-pass `fix`/`rebuild`, revert, Incident/Bug linked within N days, plus manual `aayu:correction`; `autonomy/corrections.jsonl` on `ledger` written by CI. Uses the `[notify]` sender from WP1.6. `aayu halt` sets a repo variable that every workflow and verb checks; `aayu resume` clears it.

### Completion Goal

1. [metric] state: the WP5.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: streak math at 19/20/21 and on each correction kind
2. [api-trace] state: the WP5.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `halt` stops a running loop within one poll

### Measure

WP5.7 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP5.7)

### Threshold

all 2 criteria

### Scenario

Open the WP5.7 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
