# SLO set: submitting a claim

## Journey

A member submits a claim on `/api/claims` and gets a confirmation. Reading a claim on `/api/claims/{id}` had no errors
and a p95 of 288 ms today, so it needs no SLO yet (query q1, query q2).

## Objectives

| SLO | SLI | Objective | Window | Source |
|---|---|---|---|---|
| `claim-submit-availability` | submissions without a server error | 99.5 % | 28 days | ops/slos.toml |
| `claim-submit-latency` | submissions answered within 1500 ms | 95 % | 28 days | ops/slos.toml |

Both objectives stay as defined: the last full window met them, at 99.81 % and 96.4 % (slo/status.json).

## Error budget

| SLO | Last 28 days (recorded) | Today's export | Source |
|---|---|---|---|
| `claim-submit-availability` | 62.0 % left; burn 0.4 (1h), 0.6 (6h) | 108 of 147 good; burn 53.1 | slo/status.json, query q3 |
| `claim-submit-latency` | 28.0 % left; burn 0.9 (1h), 0.8 (6h) | 93 of 147 good; burn 7.3 | slo/status.json, query q2 |

Today's "SPENT" covers 3h55m of a 28-day window and the whole of incident #7 (query q1); it is a burn rate, not the
budget. Over 28 days the window allows about 2 066 failed submissions at 99.5 % of 413 162 (slo/status.json); 785 had
failed, and today adds 39 (query q3), so about 60 % is left. Recording today's export with `--record` would show a
spent budget and stop every Story at release, so record only from a full 28-day export.

The latency budget is the tighter one: 28 % left before today, and today's 54 slow submissions spend more of it
(slo/status.json, query q2).

## Alerts

| SLO | Page | Ticket | Runbook |
|---|---|---|---|
| `claim-submit-availability` | 1h burn rate above 14.4 | 6h burn rate above 6 | ops/runbooks/claim-submit-errors.md |
| `claim-submit-latency` | none | 6h burn rate above 6 | to write (ops/slos.toml) |

Today the availability page would have fired at a 1h burn rate of 53.1 (query q3). Unverified: whether it did; the
alert history is not in the grounding.

## Budget policy

- While either budget is spent, release ships only Bugs; a Story needs an accepted `error-budget` waiver on it (ops/slos.toml).
- The Production Engineer records the budget each Monday from a full 28-day export; the Engineering Manager decides
  waivers; a second spent window in a quarter escalates to the founder (ops/slos.toml).
- At 28 % latency budget left, the eligibility timeout fix from #7 ships before new submission features
  (slo/status.json).

```toml
version = 1

[[slo]]
name = "claim-submit-availability"
service = "claims-api"
journey = "A member submits a claim and gets a confirmation"
sli = "availability"
route = "/api/claims"
objective = 99.5
window = "28d"
alert = "page when the 1h burn rate exceeds 14.4; ticket when the 6h burn rate exceeds 6"
runbook = "ops/runbooks/claim-submit-errors.md"

[[slo]]
name = "claim-submit-latency"
service = "claims-api"
journey = "A member submits a claim and gets a confirmation"
sli = "latency"
route = "/api/claims"
threshold = "1500ms"
objective = 95.0
window = "28d"
alert = "ticket when the 6h burn rate exceeds 6"
runbook = "ops/runbooks/claim-submit-slow.md"
```
