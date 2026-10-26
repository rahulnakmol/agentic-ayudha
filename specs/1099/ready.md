# rahulnakmol/agentic-ayudha#1099: Mission start and its Kanha's start record two different times, so TestMissionStartBriefsDraftingThread fails when they cross a second

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `678ca9d4926c1430691588c03cdb126333036fb39d74bdcb2c2ab6bb9b0e825e` |
| Completion Goal hash | `4b37ce9193fa7090bbe3a88203c4e0cd44c026cf2b13e3b9a13c96c044edce11` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1099/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu mission start 889` with a clock that moves one second on each read; expected: `parent.started_at` in specs/889/approved.json equals `started_at` in missions/889/plan.json
2. [api-trace] state: `go test ./internal/cli -run 'TestMissionStartBriefsDraftingThread$' -count=50`; expected: 50 of 50 pass
3. [api-trace] state: the plan of #889 already started at 10:00:00, and Start starts only its Kanha at 10:20:00 with the same moving clock; expected: the plan keeps `started_at` 10:00:00, and `parent.started_at` is 10:20:00, the one time of that Kanha start

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

The start time of a mission and the start time of its Kanha can differ by one second. `missionStart` stamps the plan's `started_at` with one `e.Now()` (internal/cli/mission.go:740), and `startKanha` stamps the parent's `StartedAt` with a second `e.Now()` (internal/cli/kanha.go:53). `TestMissionStartBriefsDraftingThread` (internal/cli/kanha_test.go:146) fails when the two calls fall on each side of a second.

### Steps to reproduce

1. Check out origin/dev at 33aa6bf4.
2. Run `go test ./internal/cli -run 'TestMissionStartBriefsDraftingThread$' -count=8`.
3. Some runs fail with `parent {... StartedAt:2026-...:41Z}, want {... StartedAt:2026-...:40Z}`.

### Expected behaviour

A mission start has one start time. `missionStart` passes the time it wrote to the plan into `startKanha`, so `specs/<f>/approved.json` `parent.started_at` equals `missions/<f>/plan.json` `started_at`. When only the Kanha starts again (the plan already started), the Kanha's start time is that start's one time.

### Actual behaviour

`startKanha` calls `e.Now()` again. The test failed 1 of 8 runs in this orb, and 2 of 8 runs on dev in the design thread.

### Completion Goal

1. [api-trace] state: `aayu mission start 889` with a clock that moves one second on each read; expected: `parent.started_at` in specs/889/approved.json equals `started_at` in missions/889/plan.json
2. [api-trace] state: `go test ./internal/cli -run 'TestMissionStartBriefsDraftingThread$' -count=50`; expected: 50 of 50 pass
3. [api-trace] state: the plan of #889 already started at 10:00:00, and Start starts only its Kanha at 10:20:00 with the same moving clock; expected: the plan keeps `started_at` 10:00:00, and `parent.started_at` is 10:20:00, the one time of that Kanha start

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Start a mission with a clock that ticks on every read; run the flaky test 50 times; start only the Kanha of a mission that already started.

### Environment

go test of internal/cli with the mission world and a fake clock (`Env.Now`, internal/cli/env.go:54); never production

### Technical Owner

engineering-manager

### Assumptions

#### Notes

The fix passes the start time as an argument to `startKanha`. It changes no record field and no message.

#### Review guidance

- Check: Criterion 1: the test clock moves on every `Now()` call, so it fails on the old code every time.
- Check: Criterion 3: the Kanha-only path still gets its own start time.
- Ignore: the wording of start's report.
- Risk: other callers of `startKanha` that also call `e.Now()`.

#### Behaviours

1. says: A mission start writes one start time to both records; rule: never; source: criteria 1, 2
````
