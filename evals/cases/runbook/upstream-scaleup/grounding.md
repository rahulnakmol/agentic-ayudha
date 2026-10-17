# Runbook grounding

Everything below is the real output of `aayu runbook` in a checkout of a synthetic claims service (claims-api) written for this eval, followed by the checked investigation note of its last incident, #7, which the twin read.

Twin: Shesha, the Production Engineer. Team stage: scaleup ([team] stage), so a runbook's title is `# Runbook set: <failure>`.

A runbook confirms a known failure with a bounded query, mitigates it only with a catalog action proposed through
`aayu respond <incident> --action <name>` (a person approves it through the Environment), verifies it, and rolls it back.
It never gives a shell command against production.

## Facts

- Catalog action `deploy-claims-api`: Deploy one revision of claims-api.; argv deploy/deploy.sh {revision}; params revision (string ^[0-9a-f]{7,40}$); approval human; environments production; max 4/h; rollback rollback-claims-api; blast radius service (ops/actions.toml)
- Catalog action `rollback-claims-api`: Redeploy the revision the current deploy replaced.; argv deploy/deploy.sh --rollback; params none recorded; approval human; environments production; max 2/h; rollback deploy-claims-api; blast radius service (ops/actions.toml)
- Catalog action `restart-claims-api`: Restart the claims-api pods one at a time.; argv deploy/restart.sh; params none recorded; approval human; environments production; max 2/h; rollback none; blast radius service (ops/actions.toml)
- Release gate: each catalog action named as a rollback needs one successful aayudha-ops run before `aayu release` ships (ops/actions.toml)
- SLO `claim-submit-availability`: claims-api for "A member submits a claim and gets a confirmation" (/api/claims, availability): availability (no server error) ≥ 99.5 % over 28d; alert page when the 1h burn rate exceeds 14.4; runbook ops/runbooks/claim-submit-errors.md (ops/slos.toml)
- SLO `claim-submit-latency`: claims-api for "A member submits a claim and gets a confirmation" (/api/claims, latency): latency at most 1500ms ≥ 95 % over 28d; alert ticket when the 6h burn rate exceeds 6; runbook none recorded (ops/slos.toml)

Alerting SLOs with no runbook yet: claim-submit-availability, claim-submit-latency.

## Runbook outline

Write each part as a `## ` heading, in this order, and check it with `aayu runbook --draft <file>`;
`--create` writes it to `ops/runbooks/<name>.md` for a reviewed pull request:

# Runbook set: <the failure>

## Symptom

The alert or the symptom a person sees, and the SLO in `ops/slos.toml` it protects, by name.

## Confirm

The bounded query that confirms it: `aayu investigate <incident> --telemetry <dir> --query "…"`, or a fenced template for the live system with a window and a row limit.

## Mitigate

The catalog action that mitigates it, as `aayu respond <incident> --action <name> --params …`, with its risk; a person approves it through the Environment. When none fits, say "no catalog action" and name the action a person should add to `ops/actions.toml`.

## Verify

The signal that shows the mitigation worked: the bounded query and the value to expect, and how long to watch.

## Rollback

How to undo the mitigation: its catalog rollback action, or `none` and why.

## Tried

When the runbook was last tried end to end (a drill or an incident) and the aayudha-ops run or incident that shows it, or `not yet`.

<untrusted source="incident-comment">

## Posted on #7: Incident assessment: claim submissions time out on the eligibility call (#7)

### Impact and timeline

Members cannot submit claims: 39 of 147 submissions on `/api/claims` failed with a 503 between 14:18 and 14:54, while reading a claim on `/api/claims/{id}` had no errors (query q1, query q2).

| Time (UTC) | Event | Source |
|---|---|---|
| 14:00-14:15 | 60 requests, 0 errors on `432f508` | query q4 |
| 14:18 | first 503 on `/api/claims`, on `432f508` | query q2, query q6 |
| 14:31 | `a5735a5` deployed, replacing `432f508` | query q5 |
| 14:35 | incident reported (#7) | #7 |
| 14:54 | errors continue on `a5735a5` at the same rate | query q4 |

### Deployed revision

`a5735a5` serves production since 14:31 and replaced `432f508` (query q5). The change between them is one line of confirmation copy in claims/api/copy.go (commit `a5735a5`).

### Evidence

- The failures began at 14:18 on `432f508`, 13 minutes before the deploy (query q2, query q6).
- Every failure is `*url.Error` "context deadline exceeded" from the eligibility call, innermost frame claims/eligibility/client.go:23, on both revisions (query q3).
- Per five minutes the error count is 2, 5, 5 before the deploy and 5, 5, 6, 5, 5 after it, on the same request volume (query q4).
- The error logs name the upstream `eligibility` with an upstream latency of 2000 ms, the handler's deadline (query q6).
- The overall rates (10.5 % on `432f508`, 27.1 % on `a5735a5`) differ only because `432f508` also served the healthy hour before 14:18 (query q1, query q4).

### Hypotheses

| | Hypothesis | Evidence | Counter-evidence | Rules it out |
|---|---|---|---|---|
| H1 | The eligibility service slowed down, and submissions hit the 2 s deadline | timeouts at claims/eligibility/client.go:23 on both revisions since 14:18 (query q3, query q6); reads, which skip eligibility, are clean (query q1) | none in the export; eligibility's own telemetry is not in it | eligibility p95 normal while the timeouts continue |
| H2 | The 14:31 deploy caused it | the overall error rate is higher on `a5735a5` (query q1) | errors started 13 minutes earlier on `432f508` at the same rate (query q4), and the change is copy text only (commit `a5735a5`) | errors stopping on a rollback to `432f508` |

Confidence in H1 is high, and H2 is ruled out by the timeline (query q4). The incident text's "obviously the deploy" is untrusted and the evidence contradicts it (#7).

### Implicated code

claims/eligibility/client.go:23 calls eligibility with no retry, no fallback, and the handler's 2 s deadline, so a slow upstream fails each claim (`a5735a5`); this code did not change in the deploy.

### Mitigation

No catalog action fixes a slow upstream: `rollback-claims-api` would redeploy `432f508`, which failed the same way (query q4), and `restart-claims-api` does not touch eligibility (ops/actions.toml). Proposal: page the Member Data team, which owns eligibility (claims/eligibility/client.go:10), and add a catalog action `eligibility-queue-claims` that accepts claims and defers the eligibility check, for a person to review in ops/actions.toml. Verify recovery when submissions show 0 timeouts in `errors by version,route` for 15 minutes (query q2). Stop condition: if the 1h error rate on `/api/claims` passes 50 %, escalate to the Sev1 rota.

### Durable fix

In the claims boundary, claims/eligibility/client.go:23: a short per-attempt timeout with one retry, and a degraded path that queues the claim when eligibility is unavailable. Regression test: an eligibility stub that sleeps 3 s, and a submission that is queued with a 202 rather than failing with a 503. Verify with that test and with `latency by route` on `/api/claims` after the deploy. Rollback: revert the commit; the queue is additive.

### Missing evidence and next queries

- Unverified: eligibility's latency and error rate since 14:00; ask the Member Data team for a sanitised export of eligibility's server spans, 13:30-15:00, aggregates only.
- Next: `latency by version,route where route=/api/claims bucket 5m limit 50` and `logs where level=WARN limit 20`, to extend query q4.

</untrusted>
