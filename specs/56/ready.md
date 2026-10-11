# rahulnakmol/agentic-ayudha#56: WP5.6 `decide`, `respond`, ops catalog

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `500ae8841efd847537514290c1f270d951b56605ea7d0856dfa3ed3863d67bc0` |
| Completion Goal hash | `251e0c46e450081973c2b202d7dac52e409a96557ac4b4e855997035b968ae0e` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/56/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the WP5.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: non-catalog command refused
2. [api-trace] state: the WP5.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: same-PR catalog edit cannot be used
3. [api-trace] state: the WP5.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: human approval required and verified
4. [api-trace] state: the WP5.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a reversible Decision fixture at 48 business hours gains `decision:provisional` with the escalation comment mentioning founder B, an irreversible one does not

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP5.6 `decide`, `respond`, ops catalog built as docs/BUILD-PLAN.md specifies, so that M5 (Seats) delivers: The twelve seats with charters and RACI, grill for every type, slice and plan, build and ship, decide and respond, autonomy and halt.

Plan text (docs/BUILD-PLAN.md, WP5.6): `decide`: options/evaluation/recommendation; `[decisions] authority.<type|layer>`; SLA in business hours (`[time]`): 24 h → `[notify]` page to the authority **and** escalation comment that @-mentions founder B's login (`[decisions] escalate_to`); 48 h → if `reversible: true` the recommended option is applied, the issue gets `decision:provisional`, the ratify/revert deadline is written to the body, and a Decision ratified later clears the label; if `reversible: false` the SLA lapse only re-pages and `status` keeps showing it overdue; stories `constrains`-linked show blocked; supersede chain. `decide --waiver <floor>` creates a floor-waiver Decision; `audit` counts waivers. `ops/actions.toml`: `{name, argv_template, params typed, approval auto|human, environments, max_per_hour, rollback, blast_radius}`; read only from the protected default branch at a pinned SHA; edits need two human CODEOWNER reviews and can never be `auto` on first merge. `respond <incident>`: untrusted telemetry wrapped; proposes a catalog action; `approval = human` executes only through a GitHub Environment with required reviewers and a short-lived prod credential never present in the agent env; timeline appended; closes with a linked Bug and retro entry.

### Completion Goal

1. [api-trace] state: the WP5.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: non-catalog command refused
2. [api-trace] state: the WP5.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: same-PR catalog edit cannot be used
3. [api-trace] state: the WP5.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: human approval required and verified
4. [api-trace] state: the WP5.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a reversible Decision fixture at 48 business hours gains `decision:provisional` with the escalation comment mentioning founder B, an irreversible one does not

### Measure

WP5.6 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP5.6)

### Threshold

all 4 criteria

### Scenario

Open the WP5.6 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
