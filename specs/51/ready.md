# rahulnakmol/agentic-ayudha#51: WP5.1 Seats, charters, RACI

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `2c36ce7b3dc77b4fcc6db0b5db3e565063b148359d61fbbe07e220e9993870ae` |
| Completion Goal hash | `35c43db2c4a8e054aec3726217e405d413c79133cae60031997465b9a8707ce6` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/51/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the WP5.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: registry lint: every verb one A, every layer one owner
2. [api-trace] state: the WP5.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `seat add` fixture produces the expected CODEOWNERS diff and onboarding issue
3. [api-trace] state: the WP5.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `seat remove` leaves no story with a dangling human owner

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP5.1 Seats, charters, RACI built as docs/BUILD-PLAN.md specifies, so that M5 (Seats) delivers: The twelve seats with charters and RACI, grill for every type, slice and plan, build and ship, decide and respond, autonomy and halt.

Plan text (docs/BUILD-PLAN.md, WP5.1): `seats/<seat>/CHARTER.md` ×12: Product Strategist, Code Architect, Engineering Manager, AI Native Engineer (+SRE hat), UX Researcher & Evolutionist, DevOps & DevEx Tinkerer, Quality Architect, Mentalist, Auditor, Marketer & Documenter, Revenue Architect (a seat; its paired human sits with founder B), Founder. RACI generated from registry: one Accountable seat per verb, one owner per layer, *executes* vs *owns threshold* distinguished (Engineer executes `verify`; Quality Architect owns `CONSTRAINTS.md`; EM owns `inspect` policy, review lane executes it; `review` owned by UX; `retro` by EM; `respond` by Engineer; `audit` by Auditor; `measure` by Revenue Architect; `brief` by Mentalist; `tell` by Marketer; `release` by DevOps). Rendered into `AGENTS.md`. **`aayu seat add|remove|list`**: `add <seat> --human <login>` switches a seat from solo to paired (writes `[seats.<seat>]`, adds the login to CODEOWNERS for the seat's layers, posts the seat's charter and "how we work" digest to the human as an onboarding issue, and when it is the second human in the org flips the WP0.1 solo-mode policy off); `remove` reverses it, reassigns open stories' Technical Owner to the twin, and records the change on `ledger/seats/`. Seat changes are protected-path writes.

### Completion Goal

1. [api-trace] state: the WP5.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: registry lint: every verb one A, every layer one owner
2. [api-trace] state: the WP5.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `seat add` fixture produces the expected CODEOWNERS diff and onboarding issue
3. [api-trace] state: the WP5.1 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `seat remove` leaves no story with a dangling human owner

### Measure

WP5.1 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP5.1)

### Threshold

all 3 criteria

### Scenario

Open the WP5.1 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
