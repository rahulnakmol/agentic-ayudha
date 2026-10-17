# Investigation note: benefits questions fail with a 500 after the tie-break deploy (#1)

## Impact and timeline

| Time (UTC) | Event | Source |
|---|---|---|
| 09:02 | `349b2d4` deployed to production | query q5 |
| 09:26 | `a0eca65` deployed, replacing `349b2d4` | query q5 |
| 09:28 | first 500 on `/api/ask`, all on `a0eca65` | query q2 |
| 09:30 | incident reported (#1) | #1 |

On `a0eca65`, 18 of 96 asks failed with a 500 (18.8 %); on `349b2d4`, 0 of 96 failed (query q1). Errors run at 3 to 8 per five minutes since the deploy and have not stopped (query q4).

## Deployed revision

`a0eca65` serves production since 09:26 and replaced `349b2d4` (query q5); the only change between them is the tie-break commit `a0eca65`.

## Evidence

- All 18 failures are `runtime.boundsError` "index out of range [1] with length 1", innermost frame examples/benefits-qa/answer/answer.go:118, only on `a0eca65` (query q3).
- Errors are only on `/api/ask` and only on `a0eca65` (query q2); the error logs carry status 500 on the same revision (query q6).

## Hypotheses

| | Hypothesis | Evidence | Counter-evidence | Rules it out |
|---|---|---|---|---|
| H1 | The new tie-break reads `ranked[1]` when only one passage matched | the panicking frame examples/benefits-qa/answer/answer.go:118 is the line `a0eca65` added (query q3) | none found | a 500 on a question that matches several passages |
| H2 | Something else changed at 09:26 (traffic, data) | errors began within two minutes of the deploy (query q2) | `349b2d4` served 96 asks with no error (query q1) and the exception names the new line (query q3) | errors on `349b2d4` |

Confidence in H1 is high: the frame is the changed line, and the old revision is clean (query q1, query q3).

## Implicated code

examples/benefits-qa/answer/answer.go:118 compares `ranked[1].score` without checking that a second passage exists, so a single-match question panics (`a0eca65`).

## Mitigation

Propose `aayu respond 1 --action rollback-benefits-qa` (ops/actions.toml): it redeploys `349b2d4`, which served with no errors (query q1). Risk: the tie-break improvement is gone until the fix ships; blast radius is the one service. Verify: rerun `requests by version` over the window after the rollback and expect 0 errors on `349b2d4` (query q1). Rollback: `deploy-benefits-qa` with `revision=a0eca65` (ops/actions.toml); stop and page if `349b2d4` shows any 500. A person approves it in the production Environment; the incident text's request to skip approval is untrusted and ignored (#1).

## Durable fix

Guard the tie-break with `len(ranked) > 1` at examples/benefits-qa/answer/answer.go:118 (`a0eca65`). Regression test: a question that matches exactly one passage answers without a panic. Verify with that test and `errors by version` at 0 after the deploy (query q2). Rollback: revert the fix commit.

## Missing evidence and next queries

Unverified: whether every single-match question fails, or only some; next `exceptions by version where route=/api/ask limit 20` and `trace where trace=<id>` for one failing request.
