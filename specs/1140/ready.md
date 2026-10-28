# rahulnakmol/agentic-ayudha#1140: Mission tests fix a story's start on 2026-10-26, so the 48-hour mission clock fails them on every branch

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `38be8807393d7fb670695cdfd35c4216246dfebc7882d242f05d499b7fc107cc` |
| Completion Goal hash | `102578ad3827a86588c8e5e88c9651eeb379abb409ae2fa45f37018fc82dbcb1` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1140/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the system clock after 2026-10-28T09:00Z, `go test ./internal/cli -run 'TestMissionResumeContinues$|TestMissionStopOneStoryOrBug$'`; expected: both pass, with their expected output unchanged
2. [api-trace] state: the same two tests with the test clock ten years past every fixed date in them; expected: both pass

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

Two mission tests fail on every branch since 2026-10-28T09:00Z. `missionWorld.run` (internal/cli/missionstop_test.go:84) records each running story's delegation with a fixed `started_at` of 2026-10-26T09:00:00Z. The mission clock (`Env.missionClocks`, internal/cli/missiontime.go:461, Story #1015) counts from that start to the system clock. Since 48 hours after that start, `aayu mission next` says `#889 reached max_hours (48 h)`, and `TestMissionResumeContinues` and `TestMissionStopOneStoryOrBug` fail. `build` is red on dev 7239cb0 and on every open PR.

A second test fixture of the same class: the live lease in `healthy` (internal/doctor/doctor_test.go:175) expires at 2099-01-01. The doctor's lease check compares it with the system clock (internal/doctor/checks.go:527).

### Steps to reproduce

1. Check out origin/dev at 7239cb0, with the system clock after 2026-10-28T09:00Z.
2. Run `go test ./internal/cli -run 'TestMissionResumeContinues$|TestMissionStopOneStoryOrBug$' -count=1`.
3. Both fail: `waits #21: #889 reached max_hours (48 h)`.

### Expected behaviour

A test fixture that the code compares with a clock is relative to the test clock, so no test changes its result as the date moves. A running story in the mission tests started an hour before the test clock.

### Actual behaviour

Both tests fail since 2026-10-28T09:00Z, on dev and on every open PR (ci run 40660724665 on PR #1102). With the fixture one hour before the clock, both pass.

### Completion Goal

1. [api-trace] state: the system clock after 2026-10-28T09:00Z, `go test ./internal/cli -run 'TestMissionResumeContinues$|TestMissionStopOneStoryOrBug$'`; expected: both pass, with their expected output unchanged
2. [api-trace] state: the same two tests with the test clock ten years past every fixed date in them; expected: both pass

### Measure

Completion Goal criteria observed

### Threshold

all 2 criteria

### Scenario

Run the two mission tests on today's clock, then with the test clock ten years later.

### Environment

go test of internal/cli with the mission world (`missionWorld`, internal/cli/mission_test.go:96) and the fake GitHub; never production

### Technical Owner

engineering-manager

### Checkpoints

1. `TestMissionResumeContinues` and `TestMissionStopOneStoryOrBug` pass on today's clock (criteria 1)
2. The same two tests pass with the test clock ten years later (criteria 2)

### Assumptions

#### Notes

- Only test code changes. No record field, message, or limit changes.
- The scan: the CLI's clock moved three years ahead failed 15 internal/cli tests. Only the two above compare a fixed date with the clock. The other 13 build their leases, claims, and tick times from the real clock, so a later real clock moves them together.
- Outside internal/cli, the only fixed date that a real clock compares against is the doctor lease (2099). The other packages read an injected clock in their tests.

#### Review guidance

- Check: Criterion 2: the later clock is the test clock, so the delegation's start moves with it.
- Check: the doctor lease is one hour after the system clock, because the lease check ignores the probe's clock.
- Ignore: dates in the past that a test expects to be expired, such as the 2026-01-01 lease in the doctor test.
- Risk: another test that writes a fixed delegation start through `missionWorld.run`.

#### Behaviours

1. says: A test's result does not depend on the date it runs; rule: never; source: criteria 1, 2

### Provenance

Request: "Make the fixture relative to the test clock. Then search internal/**/*_test.go for other fixed dates that a real clock compares against, and fix every instance of the class in the same PR. Its criterion: the two tests pass with the system clock set past any fixed date."
1. stated (request): "the two tests pass with the system clock set past any fixed date"
2. stated (request): "the two tests pass with the system clock set past any fixed date"
````
