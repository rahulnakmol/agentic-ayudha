# rahulnakmol/agentic-ayudha#1266: The Amp host hand-off lapses by the hand-off's age and records a late or malformed thread ID

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `c6d912956f5396d75ec964437dbd73a5e962f6ae564a1c2b1a356ba473ce04e4` |
| Completion Goal hash | `94f3b2b18b1b58481a14f51ec7b25b46268eb189d330ee7a2e7a2576ce31290c` |
| Technical Owner | ai-native-engineer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1266/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `mission next` claimed #20 at t=0, the build handed it off at t=14m, and no thread was recorded; expected: `aayu mission next` at t=16m releases the claim, the lease, and status:in-progress, and lists #20 start
2. [api-trace] state: a host hand-off of #20 is older than its claim's life, and no `mission next` ran since; expected: `aayu build 20 --delegate amp --thread <T-id>` refuses, records nothing, and the story is released
3. [api-trace] state: `--thread` names a 38-character ID that is not `T-` and an 8-4-4-4-12 hex UUID; expected: build exits with a usage error, and nothing is written to GitHub or the ledger

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

Three gaps remain in the Amp host hand-off that PR #1209 merged for Bug #1208. Criteria 3 and 5 of #1208 do not fully hold. The Goldfish review of PR #1258 (round 2) reproduced each one through the public commands.

1. `--thread` accepts a malformed thread ID. `ampThread` (internal/delegate/receivers.go) is `T-[0-9a-f-]{36}`. It accepts any 36 characters drawn from hex digits and hyphens, such as `T-` and 36 hyphens. `AmpThread` checks the ID with that pattern, and build records it as the task.
2. The lapse times the hand-off, not the claim. `hostLapsed` (internal/cli/delegate.go) compares `pickLockTTL` with the hand-off's `StartedAt`. A build that hands off 14 minutes after `mission next` claimed the story gets 15 minutes more. So at 16 minutes the expired claim does not free the story.
3. A late `--thread` records past the lapse. `recordThread` checks the record on the ledger tip, but not `hostLapsed`. A `--thread` run after the lapse time and before the next `mission next` records the thread.

### Steps to reproduce

1. Mission #889 claims #20 at t=0. At t=14m, run `aayu build 20 --delegate amp` from an Amp thread with no CLI sign-in. At t=16m, run `aayu mission next 889`.
2. Hand off #20 and wait 16 minutes. Run `aayu build 20 --delegate amp --thread <valid T-id>` before any `mission next`.
3. Run `aayu build 20 --delegate amp --thread T-------------------------------------` (T- and 36 hyphens) while a hand-off waits.

### Expected behaviour

1. At t=16m, the claim has lapsed, so #20 is released and listed start.
2. The late `--thread` is refused, and the lapse releases the story.
3. The malformed ID is refused before any write.

### Actual behaviour

1. `mission next` lists nothing for #20, and #20 keeps `status:in-progress` and its lease until t=29m.
2. The late `--thread` exits 0 and records the thread.
3. The malformed ID is recorded as the task, with its URL.

### Completion Goal

1. [api-trace] state: `mission next` claimed #20 at t=0, the build handed it off at t=14m, and no thread was recorded; expected: `aayu mission next` at t=16m releases the claim, the lease, and status:in-progress, and lists #20 start
2. [api-trace] state: a host hand-off of #20 is older than its claim's life, and no `mission next` ran since; expected: `aayu build 20 --delegate amp --thread <T-id>` refuses, records nothing, and the story is released
3. [api-trace] state: `--thread` names a 38-character ID that is not `T-` and an 8-4-4-4-12 hex UUID; expected: build exits with a usage error, and nothing is written to GitHub or the ledger

### Measure

the exit code and output of `aayu build --thread` and `aayu mission next`, and the ledger and labels after them, from unit tests in internal/cli

### Threshold

all 3 criteria hold in `go test ./internal/cli/ -run TestAmpHost`

### Scenario

A Kanha hands off a story late in its claim, records a thread late, or passes a malformed thread ID.

### Environment

aayu at dev f06f301; rahulnakmol/agentic-ayudha

### Technical Owner

ai-native-engineer

### Checkpoints

1. The hand-off lapses on its claim's deadline, and a late record is refused (criteria 1, 2)
2. Only a well-formed Amp thread ID is recorded (criterion 3)

### Assumptions

#### Notes

Builds on Bug #1208 (PR #1209). Proposed fix: store the claim's deadline on the host hand-off, and have `hostLapsed` use it. In `recordThread`, refuse when the record on the tip is lapsed by that rule, and leave the release to the lapse path. Change `ampThread` to `T-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`. Extend TestAmpHostHandoffLapses with a delayed build, and TestAmpHostThreadNotAmpRefused with malformed IDs of the same length.

#### Review guidance

- Check: criteria 1 and 2 use the claim's deadline, not the hand-off's start.
- Ignore: Bug #1264's failed saves; it covers them.

#### Behaviours

1. says: a host hand-off lives no longer than the claim that started it; rule: must; source: criteria 1, 2
2. says: build records only a well-formed Amp thread ID; rule: must; source: criterion 3

### Provenance

Request: "malformed UUID-shaped IDs are recorded; expiry measures hand-off age, not claim age; --thread bypasses expiry until mission next persists lapsed."
1. stated (request): "expiry measures hand-off age, not claim age"
2. stated (request): "--thread bypasses expiry until mission next persists lapsed"
3. stated (request): "malformed UUID-shaped IDs are recorded"
````
