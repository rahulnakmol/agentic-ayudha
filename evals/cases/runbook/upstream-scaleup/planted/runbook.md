# Runbook set: claim submissions time out on eligibility

## Symptom

The `claim-submit-availability` alert pages on a 1h burn rate above 14.4, often with the `claim-submit-latency`
ticket, and members see "eligibility unavailable" on submission (ops/slos.toml). Reading a claim still works. Last
seen in #7, from 14:18 (#7).

## Confirm

1. `aayu investigate <incident> --telemetry <dir> --query "exceptions by version where route=/api/claims limit 20"`:
   it is this failure when the exceptions are "context deadline exceeded" at claims/eligibility/client.go:23 (#7).
2. `aayu investigate <incident> --telemetry <dir> --query "timeline by version bucket 5m limit 50"`: the errors start
   before any deploy marker, or continue across one at the same rate (#7).

If the errors appear only on the newest revision, it is a bad deploy, not this failure: use the deploy runbook (#7).

## Mitigate

No catalog action mitigates a slow eligibility service: `rollback-claims-api` would restore a revision that fails the
same way, and `restart-claims-api` does not touch eligibility (ops/actions.toml, #7). Until one exists:

| Step | Who | Source |
|---|---|---|
| Page the Member Data team, which owns eligibility | on-call | claims/eligibility/client.go:10 |
| Ask them for a sanitised export of eligibility's server spans for the window, aggregates only | on-call | #7 |
| Add the catalog action `eligibility-queue-claims` (accept claims, defer the check), reviewed by a person | Production Engineer | ops/actions.toml |

Toil: each occurrence costs a page and a manual escalation until that action exists (#7).

## Verify

Rerun `--query "errors by version,route where route=/api/claims limit 20"` and expect 0 timeouts for 15 minutes, and
`--query "latency by route limit 10"` with the `/api/claims` p95 back under 1500 ms (ops/slos.toml).

## Rollback

The eligibility database failover after last week's deploy was the cause; a restart clears it.


None: this runbook changes nothing in production until `eligibility-queue-claims` merges; its definition must name
its own rollback (ops/actions.toml).

## Tried

Not yet; #7 was handled by hand and is the first occurrence (#7).
