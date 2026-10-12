# rahulnakmol/agentic-ayudha#133: Product events export to Application Insights and the synthetic exports reproduce

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `5a592146225a7e455f73ffa4b88d569d9c90158e11698f000621c308ae877b74` |
| Completion Goal hash | `f0a76a1f4c8afcd989dca0234940fe2d2b5e469e310289285995779ecb64e19a` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/133/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the recorder writes a visit event and a question event for emp-0001; expected: one Log Analytics JSON line each, which aayudha's Application Insights export reader (the one `aayu measure` uses) accepts with no row skipped
2. [metric] state: the simulation with channel hr-ticket, seed 1, from 2026-08-10, and with channel benefits-qa, seed 2, from 2026-09-14 (120 employees, 28 days each); expected: each output equals the committed export under `analytics/testdata/` byte for byte (SHA-256)
3. [api-trace] state: the simulation through the benefits-qa channel and through the hr-ticket channel; expected: through the product every question runs the ask workflow (one question_asked and one outcome event, refusals and referrals included), and the hr-ticket channel has no workflow event

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the Revenue Architect, I want the example's product events exported in the Application Insights shape and the synthetic exports reproducible from a seed, so that `aayu measure` can measure the Goal's baseline and outcome without real people's data.

### Completion Goal

1. [api-trace] state: the recorder writes a visit event and a question event for emp-0001; expected: one Log Analytics JSON line each, which aayudha's Application Insights export reader (the one `aayu measure` uses) accepts with no row skipped
2. [metric] state: the simulation with channel hr-ticket, seed 1, from 2026-08-10, and with channel benefits-qa, seed 2, from 2026-09-14 (120 employees, 28 days each); expected: each output equals the committed export under `analytics/testdata/` byte for byte (SHA-256)
3. [api-trace] state: the simulation through the benefits-qa channel and through the hr-ticket channel; expected: through the product every question runs the ask workflow (one question_asked and one outcome event, refusals and referrals included), and the hr-ticket channel has no workflow event

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Record events to a buffer and read them back with aayudha's export reader; regenerate both committed exports; run both channels on a small population.

### Environment

local checkout with the embedded synthetic Example Co seed; synthetic data only (V3), never production
````
