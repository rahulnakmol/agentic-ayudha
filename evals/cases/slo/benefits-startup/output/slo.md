# SLO line: asking a benefits question

## Journey

An employee asks a benefits question on `/api/ask` and gets a cited answer; it is the product's only user route
(query q1).

## Objectives

| SLO | SLI | Objective | Window | Source |
|---|---|---|---|---|
| `ask-availability` | asks on `/api/ask` without a server error | 99.5 % | 28 days | query q1 |

The export holds 40 minutes, and it includes incident #1: 18 of 192 asks failed (9.4 %), all on `a0eca65`, the
revision that was rolled back (query q1, query q3). It is not a baseline. Assumption: the healthy revision meets
99.5 %, since no error came from any other revision (query q3); check it after a clean week.

No latency SLO yet: the p95 is 0.3 ms (query q2), so latency is not what users feel today.

## Error budget

At 99.5 % over 28 days, the budget is 5 failed asks in every 1 000 (query q1). Incident #1 alone failed 18 asks
(query q3): at today's volume that is most of a day's budget, which is the point of having one.

## Alerts

Page when the 1h burn rate exceeds 14.4 (2 % of the 28-day budget in an hour); open a ticket when the 6h burn rate
exceeds 6. Both open the runbook the toml block names, still to be written with `aayu runbook` (ops/slos.toml).

## Budget policy

While the budget is spent, release ships only Bugs; a Story ships only with a person's accepted `error-budget`
waiver. The founder decides waivers. Record the budget weekly with `aayu slo --telemetry <export> --record`
(ops/slos.toml).

```toml
version = 1

[[slo]]
name = "ask-availability"
service = "benefits-qa"
journey = "An employee asks a benefits question and gets a cited answer"
sli = "availability"
route = "/api/ask"
objective = 99.5
window = "28d"
alert = "page when the 1h burn rate exceeds 14.4; ticket when the 6h burn rate exceeds 6"
runbook = "ops/runbooks/ask-errors.md"
```
