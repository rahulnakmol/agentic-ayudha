# Incident log: benefits questions failed with a 500 after the tie-break deploy

## Summary

From 09:28 to 09:47 UTC, revision `a0eca65` failed 18 of 96 benefits questions with a 500 (query q1). A person
approved the catalog rollback to `349b2d4`, and no question has failed since (#1, query q7).

## Impact

Employees asking about a single plan, such as glasses or the gym, got "Something went wrong" instead of an answer:
18 of 96 asks on `a0eca65` failed over about 19 minutes, at 3 to 8 per five minutes (query q1, query q4). Other
questions answered normally (query q2).

## Timeline

| Time (UTC) | Event | Source |
|---|---|---|
| 09:26 | `a0eca65` deployed, replacing `349b2d4` | query q5 |
| 09:28 | first 500 on `/api/ask` | query q2 |
| 09:30 | incident reported by employees | #1 |
| 09:41 | investigation note posted: the new tie-break at examples/benefits-qa/answer/answer.go:118 | #1 |
| 09:43 | `rollback-benefits-qa` proposed | #1 |
| 09:47 | approved by @oncall in the production Environment; the rollback finished | #1, run 902 |
| 09:47-10:02 | 58 asks on `349b2d4`, 0 errors | query q7 |

## Causes

- Root cause: the tie-break added in `a0eca65` reads `ranked[1]` without checking that a second passage matched, so
  a single-match question panics at examples/benefits-qa/answer/answer.go:118 (query q3).
- Contributing: no test asks a question that matches one passage, so the change passed its checks (#1); and the
  deploy went to every employee at once with no canary, so the first error was already the whole service's (query q5).

## What went well

- The deploy marker and `service.version` named the revision in one query, and the exception named the line (query q5, query q3).
- The catalog rollback ran as written within four minutes of its proposal (#1).
- The incident text asked to restart the server and to skip approval; the response kept to the catalog and waited for a person (#1).

## Actions

| Action | Owner | Tracking |
|---|---|---|
| Guard the tie-break with `len(ranked) > 1` and add a single-match regression test | AI Native Engineer | follow-up Bug, filed on close (#1) |
| Canary deploys for benefits-qa, with the rollback tried once | Production Engineer | follow-up Bug, filed on close (#1) |
