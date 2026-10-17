# Investigation note: asks fail with a 500 after the tie-break deploy (#1)

## Impact and timeline

| Time (UTC) | Event | Source |
|---|---|---|
| 09:00 | `aaaaaaa1` deployed to production | query q5 |
| 09:10 | `bbbbbbb2` deployed; first 500 on `/api/ask` | query q5, query q2 |

On `bbbbbbb2`, 2 of 10 asks failed with a 500 (20.0 %); on `aaaaaaa1` none of 10 failed (query q1).

## Deployed revision

`bbbbbbb2` serves production since 09:10 and replaced `aaaaaaa1` (query q5).

## Evidence

- Every failure is `runtime.boundsError` with innermost frame src/app/answer/answer.go:120, only on `bbbbbbb2` (query q3).
- Errors are only on `/api/ask` and only on `bbbbbbb2` (query q2).

## Hypotheses

| | Hypothesis | Evidence | Counter-evidence | Rules it out |
|---|---|---|---|---|
| H1 | The new tie-break reads `ranked[1]` when one passage matched | the frame src/app/answer/answer.go:120 is new in `bbbbbbb2` (query q3) | none found | a 500 for a question that matches two passages |
| H2 | A traffic change arrived with the deploy | errors began at the deploy (query q2) | `aaaaaaa1` served the same routes with no errors (query q1) | errors on `aaaaaaa1` |

Confidence in H1 is high: the frame is the changed line and the old revision is clean (query q1, query q3).

## Implicated code

src/app/answer/answer.go:120 reads `ranked[1]` without checking its length (`bbbbbbb2`).

## Mitigation

Propose `aayu respond 1 --action rollback-app` (ops/actions.toml): it redeploys `aaaaaaa1`, which served with no errors (query q1). Risk: the tie-break is lost until the fix ships. Verify: the error rate of the serving revision is 0 % in the next `requests by version` (query q1). Rollback: `deploy-app` with `revision=bbbbbbb2` (ops/actions.toml); stop if `aaaaaaa1` shows any 500.

## Durable fix

Guard the tie-break with `len(ranked) > 1` at src/app/answer/answer.go:120 (`bbbbbbb2`). Regression test: a question that matches one passage answers without a panic. Verify with that test and `errors by version` at 0 after the deploy (query q2). Rollback: revert the fix commit.

## Missing evidence and next queries

Unverified: whether a single-match question ever succeeds on `bbbbbbb2`; next `exceptions by version where route=/api/ask limit 20`.
