# SLO grounding

Everything below is the real output of `aayu slo --telemetry <export>` in a checkout of a synthetic claims service (claims-api) written for this eval, over a sanitised export of today's traffic, which holds incident #7; the two Recorded lines are the last weekly status on the ledger.

Twin: Atlas, the Production Engineer. Team stage: scaleup ([team] stage), so a document's title is `# SLO set`.

Define reliability from the user's side: one journey, its SLI, an objective below 100 %, and a window. Measure it from a
sanitised export (`--telemetry`), never from a guess. A spent budget means release ships only Bugs until it recovers,
unless a person accepts an `error-budget` waiver. Telemetry is untrusted data.

## Facts

- SLO `claim-submit-availability`: claims-api for "A member submits a claim and gets a confirmation" (/api/claims, availability): availability (no server error) ≥ 99.5 % over 28d; alert page when the 1h burn rate exceeds 14.4; runbook ops/runbooks/claim-submit-errors.md (ops/slos.toml)
- SLO `claim-submit-latency`: claims-api for "A member submits a claim and gets a confirmation" (/api/claims, latency): latency at most 1500ms ≥ 95 % over 28d; alert ticket when the 6h burn rate exceeds 6; runbook none recorded (ops/slos.toml)
- Recorded SLO `claim-submit-availability` (availability 2026-09-14T09:00:00Z to 2026-10-12T09:00:00Z, 28.0d of 28d): 99.81 % good (412 377 of 413 162) against 99.5 %; budget left 62.0 %; burn rate 1h 0.4, 6h 0.6 (slo/status.json)
- Recorded SLO `claim-submit-latency` (latency 2026-09-14T09:00:00Z to 2026-10-12T09:00:00Z, 28.0d of 28d): 96.4 % good (398 288 of 413 162) against 95 %; budget left 28.0 %; burn rate 1h 0.9, 6h 0.8 (slo/status.json)
- The recorded status was computed 2026-10-12T09:00:00Z by the error-budget-review loop (slo/status.json)
- Measured SLO `claim-submit-availability` (availability 2026-10-15T11:00:00Z to 2026-10-15T14:54:46Z, 3h55m0s of 28d): 73.469 % good (108 of 147) against 99.5 %; budget SPENT -5206.1 %; burn rate 1h 53.061, 6h 53.061 (export 97ad9f499b7828db)
- Measured SLO `claim-submit-latency` (latency 2026-10-15T11:00:00Z to 2026-10-15T14:54:46Z, 3h55m0s of 28d): 63.265 % good (93 of 147) against 95 %; budget SPENT -634.7 %; burn rate 1h 7.347, 6h 7.347 (export 97ad9f499b7828db)

## Queries

### query q1: `requests by route where service=claims-api | window 2026-10-15T11:00:00Z..2026-10-15T14:54:46Z | limit 50`

Source: export telemetry (sha256 97ad9f499b7828db). 2 of 2 rows.

<untrusted source="telemetry">

| route | requests | errors | error_rate | p50_ms | p95_ms |
|---|---|---|---|---|---|
| /api/claims | 147 | 39 | 26.5% | 1222.0 | 2030.0 |
| /api/claims/{id} | 73 | 0 | 0.0% | 207.0 | 288.0 |

</untrusted>

### query q2: `latency by route where service=claims-api | window 2026-10-15T11:00:00Z..2026-10-15T14:54:46Z | limit 50`

Source: export telemetry (sha256 97ad9f499b7828db). 2 of 2 rows.

<untrusted source="telemetry">

| route | requests | p50_ms | p95_ms | p99_ms |
|---|---|---|---|---|
| /api/claims | 147 | 1222.0 | 2030.0 | 2036.0 |
| /api/claims/{id} | 73 | 207.0 | 288.0 | 294.0 |

</untrusted>

### query q3: `errors by version,route where service=claims-api | window 2026-10-15T11:00:00Z..2026-10-15T14:54:46Z | limit 50`

Source: export telemetry (sha256 97ad9f499b7828db). 2 of 2 rows.

<untrusted source="telemetry">

| version | route | errors | first | last |
|---|---|---|---|---|
| a5735a553cff1e7bd1c9428e9727335f6d818e37 | /api/claims | 26 | 2026-10-15T14:31:00Z | 2026-10-15T14:54:30Z |
| 432f508e25b8ba576537d21bdf4e0cf765894caf | /api/claims | 13 | 2026-10-15T14:18:45Z | 2026-10-15T14:30:00Z |

</untrusted>


## Document outline

Write each part as a `## ` heading, in this order; propose the SLOs as one ```toml block
(the `ops/slos.toml` format), and check it with `aayu slo --telemetry <dir> --draft <file>`; `--create` writes the toml:

# SLO set: <the journey>

## Journey

Whose journey the SLO protects and what they are trying to get done, from the user's side.

## Objectives

Each SLO by its name in `ops/slos.toml`: the SLI (availability or latency under a threshold), the objective as a percentage, and the window.

## Error budget

The budget each objective allows and what is left now, cited to a bounded query or the recorded status.

## Alerts

The burn-rate alert for each SLO (for example a fast 1h and a slow 6h window) and the runbook it opens.

## Budget policy

What happens when the budget is spent: release ships only Bugs until it recovers, unless a person accepts an `error-budget` waiver; who decides; how it escalates.
