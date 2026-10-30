# rahulnakmol/agentic-ayudha#1186: TestMissionEndOnlyByStarter fails when the minute changes during the test

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `a2ce4c7c914766a0f3ee5148dd7495302baadd4bf36df534441d5a7ecc22e211` |
| Completion Goal hash | `443033a1e58409a87329aaaa098acef6fa2fd40dc60e1d5bdea0a3bdafa191b0` |
| Technical Owner | quality-architect |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1186/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `TestMissionEndOnlyByStarter` runs with the wall clock at hh:mm:59, so the minute changes during the run; expected: it passes, because the comment time and the expected time come from one clock
2. [api-trace] state: the end comment names a time other than the clock's; expected: the test fails at "#951's comment", as now

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

`TestMissionEndOnlyByStarter` (internal/cli/missionstop_test.go) fails when the minute changes during the test. The code writes the end comment with `e.Now().Local().Format("15:04")` (internal/cli/missionstop.go:601). The test builds the expected comment later with `time.Now().Format("15:04")` (missionstop_test.go:734). The test runs for about 10 s first: a refusal, then eight failed `--end` runs. When a minute boundary falls between the comment and the check, the HH:MM differs and the test fails. It failed so in the merge gate of PR #1183 at 9569ee8 (run 41117568809), and the coverage floor of that PR failed with 38 of 38 lines covered. The same head passed in CI `build`, in the full local gate, and three times locally. Clock-dependent tests are on the friction list of Decision #1168 (the founder's day), under red dev (#1140).

### Steps to reproduce

1. Run `go test ./internal/cli/ -run 'TestMissionEndOnlyByStarter$'` so that the `--end` comment is written in one minute and the check runs in the next.
2. See the test fail at "#951's comment".

### Expected behaviour

The test passes at any wall-clock time. It still fails when the end comment names a time other than the clock's.

### Actual behaviour

The test fails when a minute boundary falls between the end comment and its check.

### Completion Goal

1. [api-trace] state: `TestMissionEndOnlyByStarter` runs with the wall clock at hh:mm:59, so the minute changes during the run; expected: it passes, because the comment time and the expected time come from one clock
2. [api-trace] state: the end comment names a time other than the clock's; expected: the test fails at "#951's comment", as now

### Measure

the result of `TestMissionEndOnlyByStarter`

### Threshold

both criteria hold in `go test ./internal/cli/ -run 'TestMissionEndOnlyByStarter$'`

### Scenario

The merge gate or a local gate runs the `internal/cli` tests at any time of day.

### Environment

aayu built from dev b7b36fd; rahulnakmol/agentic-ayudha

### Technical Owner

quality-architect

### Checkpoints

1. The test reads the comment time from the same clock as the code (criteria 1, 2)

### Assumptions

#### Notes

The code already reads its clock through `e.Now`, so the test can fix it to one instant. Change only the test, not the end comment.

#### Review guidance

- Check: the test still fails when the comment names another time.
- Ignore: the rest of the mission stop test.

#### Behaviours

1. says: A test result does not depend on the wall-clock minute; rule: must; source: criteria 1, 2

### Provenance

Request: "TestMissionEndOnlyByStarter compares the comment's HH:MM with time.Now() after a run of about 10 s"
1. stated (request): "TestMissionEndOnlyByStarter compares the comment's HH:MM with time.Now() after a run of about 10 s"
2. default (scope): "change only the test, not the end comment"
Term: "HH" → the hour of the comment's time, two digits (Go layout "15"); "the comment's HH:MM"
Term: "MM" → the minute of the comment's time, two digits (Go layout "04"); "the comment's HH:MM"
````
