# Postmortem: claim submissions timed out while eligibility was slow

## Summary

From 14:18 to about 15:04 UTC, 39 of 147 claim submissions failed with a 503 because claims-api's eligibility call
hit its 2 s deadline (query q1, query q3). The eligibility service's owners reverted a connection-pool change at
15:04, and submissions have had no errors since (#7, query q7). No catalog action ran: none addresses a slow
dependency (#7).

## Impact

- Members could not submit claims for about 46 minutes (14:18 to 15:04); 39 of 147 submissions failed and saw
  "eligibility unavailable" (query q1, query q2, #7).
- Reading an existing claim was unaffected (query q1).
- Detection: members reported it at 14:35, 17 minutes after the first failure; no alert fired (#7, query q2).
- Response: the investigation note was posted at 14:44 and the owners paged at 14:48 (#7).

## Timeline

| Time (UTC) | Event | Source |
|---|---|---|
| 14:10 | eligibility connection pool lowered from 40 to 20 (Member Data team) | #7 |
| 14:18 | first 503 on `/api/claims`, on `432f508` | query q2 |
| 14:31 | unrelated copy change `a5735a5` deployed | query q5 |
| 14:35 | incident reported by members | #7 |
| 14:44 | investigation note posted: timeouts at claims/eligibility/client.go:23 on both revisions; the deploy ruled out | #7 |
| 14:48 | Member Data team paged; no catalog action fits, so claims-api left unchanged | #7 |
| 15:04 | eligibility pool change reverted by its owners | #7 |
| 15:05-15:25 | 84 submissions, 0 errors, p95 242 ms | query q7 |

## Causes

- Root cause: @lee broke eligibility with a configuration change that halved its database connection pool, so under the
  afternoon load its p95 rose from 180 ms to about 2.4 s, past the 2 s deadline of claims-api's call (#7, query q6).
- Contributing: claims/eligibility/client.go:23 calls eligibility with the handler's whole deadline, no retry, and
  no degraded path, so a slow dependency fails every affected claim (#7).
- Contributing: neither service alerted. The submission SLO had no burn-rate alert and eligibility had no
  pool-saturation alert, so members detected it first (#7, query q2).
- Contributing: the incident text named the 14:31 deploy as the cause; the timeline showed the errors began 13 minutes before
  it, on the old revision, which kept a rollback that could not help from being proposed (query q4, #7).

## What went well

- The deploy marker and the five-minute timeline ruled out the deploy in one query (query q4).
- No restart or rollback was run against claims-api for a cause outside it, despite two requests to (#7).
- The eligibility owners found and reverted the change within 18 minutes of the page (#7).

## Actions

| Action | Owner | Tracking |
|---|---|---|
| Short per-attempt timeout, one retry, and a queued 202 path when eligibility is unavailable, with a 3 s stub regression test | AI Native Engineer | follow-up Bug, filed on close (#7) |
| Burn-rate alerts and a runbook for the claim-submission SLOs | Production Engineer | Story, linked on #7 |
| Catalog action `eligibility-queue-claims`, reviewed by a person in ops/actions.toml | Production Engineer | Story, linked on #7 |
| Pool-saturation alert and load check for eligibility config changes | Member Data team, to confirm on #7 | asked on #7 |
