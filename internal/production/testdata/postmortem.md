# Incident log: asks failed with a 500 after the tie-break deploy

## Summary

From 09:10 to 09:42 UTC, 2 of every 10 benefits questions failed with a 500 on revision `bbbbbbb2`; a person approved
the rollback to `aaaaaaa1` and the errors stopped (#1).

## Impact

Members asking single-match questions got an error instead of an answer: 2 of 10 asks failed on `bbbbbbb2` (query q1).

## Timeline

| Time (UTC) | Event | Source |
|---|---|---|
| 09:10 | `bbbbbbb2` deployed; first 500 on `/api/ask` | #1, query q5 |
| 09:20 | investigation note posted; rollback proposed | #1 |
| 09:31 | `rollback-app` approved by a reviewer and finished success | #1 |
| 09:42 | no 500 on `aaaaaaa1` since the rollback | query q1 |

## Causes

- Root cause: the tie-break reads the second ranked passage without checking there is one, src/app/answer/answer.go:120
  in `bbbbbbb2`.
- Contributing: the tests cover only questions with several matching passages (src/app/answer/answer.go:120), and the
  deploy had no canary, so every member met the new revision at once (#1).

## What went well

- The deploy marker and `service.version` named the revision in one query (query q5).
- The catalog rollback ran as written (#1).

## Actions

| Action | Owner | Tracking |
|---|---|---|
| Guard the tie-break and add a single-match regression test | AI Native Engineer | follow-up Bug, filed on close (#1) |
| Canary deploys for benefits-qa | Production Engineer | #1 |
