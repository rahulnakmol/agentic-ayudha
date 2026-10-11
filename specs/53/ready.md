# rahulnakmol/agentic-ayudha#53: WP5.3 `grill`

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `3dabe05b08759e4cb30a73b326bc58f33bafaf3565df5eeba210d8969fb9faac` |
| Completion Goal hash | `fb21739a0d9e577ed8f4300eda019a7716b61fedddc34598bd1d40d1e70890bf` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/53/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the WP5.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: tier-1 eval per type (10 fixtures, one per type) ends in an issue passing `ready` 10/10
2. [metric] state: the WP5.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: tier-2 rubric on `grill goal` and `grill story` meets the WP3.4 bar (pass^5 ≥ 0.8, mean ≥ 3.5) and the result file is committed as the baseline
3. [api-trace] state: the WP5.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a Goal fixture with no reachable baseline is refused Ready with the message naming the missing metric

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP5.3 `grill` built as docs/BUILD-PLAN.md specifies, so that M5 (Seats) delivers: The twelve seats with charters and RACI, grill for every type, slice and plan, build and ship, decide and respond, autonomy and halt.

Plan text (docs/BUILD-PLAN.md, WP5.3): Question sets per type in `references/<type>.md` keyed to schema sections; baseline required before a target on Goal/Initiative; refuses Ready without validator pass; writes via `actions` under the human's identity when run by a human, bot otherwise; `grill brand` → `design/brand.yaml` (AA palette check); TUI (bubbletea) for CLI, prompt for MCP/skill; untrusted-input wrapping of any pasted material. Extends the WP4.10 markdown `grill story` in place.

### Completion Goal

1. [api-trace] state: the WP5.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: tier-1 eval per type (10 fixtures, one per type) ends in an issue passing `ready` 10/10
2. [metric] state: the WP5.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: tier-2 rubric on `grill goal` and `grill story` meets the WP3.4 bar (pass^5 ≥ 0.8, mean ≥ 3.5) and the result file is committed as the baseline
3. [api-trace] state: the WP5.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a Goal fixture with no reachable baseline is refused Ready with the message naming the missing metric

### Measure

WP5.3 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP5.3)

### Threshold

all 3 criteria

### Scenario

Open the WP5.3 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
