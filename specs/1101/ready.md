# rahulnakmol/agentic-ayudha#1101: aayu halt leaves mission claims in place, so a story claimed just before a halt still counts as claimed after aayu resume

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `d2c6826c0fd5616e5261cc238ae10404efd0489d4d3e5f1b2311cf1880f6abcd` |
| Completion Goal hash | `762475e95a067031f870e7b8eb318fb009fb23922dd60dd8262de96366e756ed` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1101/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: #20 of #889 and #930 of #920 each hold a live claim with no thread started, and a person runs `aayu halt --reason "release cut"`; expected: missions/889/claims.jsonl and missions/920/claims.jsonl each gain a release line for that claim, and the halt report says `released 2 mission claims (#20, #930)`
2. [api-trace] state: `aayu resume` at 10:08, then `aayu mission next 889`; expected: it marks #20 start and writes a new claim line, after the same checks as any start
3. [api-trace] state: after the halt, `aayu build 20 --delegate amp` runs with the claim from 10:00; expected: it refuses with `the claim of #20 was released by aayu halt at 10:05: run aayu mission next 889` and starts nothing
4. [api-trace] state: #21's thread was already running when the halt came; expected: after resume `aayu mission next 889` lists #21 as running and never starts it again
5. [api-trace] state: the ledger cannot be written during `aayu halt`; expected: AAYUDHA_HALT is still set, and the report says `mission claims not released: <reason>; they expire within 15 minutes`

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

`aayu halt` leaves mission claims in place. `missionNext` claims each start in `missions/<f>/claims.jsonl` (`MissionClaimsPath`, internal/cli/missionnext.go:63), and a claim stays live for `pickLockTTL`, 15 minutes (internal/cli/buildnext.go:219, `liveClaims` :343-361). `runHalt` sets AAYUDHA_HALT and writes no claim line (internal/cli/halt.go:266-298). So a story claimed less than 15 minutes before a halt still counts as claimed after `aayu resume`, and a build with that claim skips the checks a new start runs. CodeRabbit raised it on PR #1095 (Major, CWE-863), and @rahulnakmol accepted it for #1005.

### Steps to reproduce

1. `aayu mission next 889` marks #20 start at 10:00 and writes its claim line.
2. A person runs `aayu halt --reason "release cut"` at 10:05, before #20's thread starts.
3. The person runs `aayu resume` at 10:08.
4. `aayu mission next 889` lists `waits #20: claimed by another run`, and `aayu build 20 --delegate <tool>` with the old claim goes ahead.

### Expected behaviour

Halt releases every live mission claim. After resume, `aayu mission next` reads the live state and marks those stories start again, with a new claim.

### Actual behaviour

The claim from before the halt stays live until 10:15, and a build that holds it bypasses the new start's checks.

### Completion Goal

1. [api-trace] state: #20 of #889 and #930 of #920 each hold a live claim with no thread started, and a person runs `aayu halt --reason "release cut"`; expected: missions/889/claims.jsonl and missions/920/claims.jsonl each gain a release line for that claim, and the halt report says `released 2 mission claims (#20, #930)`
2. [api-trace] state: `aayu resume` at 10:08, then `aayu mission next 889`; expected: it marks #20 start and writes a new claim line, after the same checks as any start
3. [api-trace] state: after the halt, `aayu build 20 --delegate amp` runs with the claim from 10:00; expected: it refuses with `the claim of #20 was released by aayu halt at 10:05: run aayu mission next 889` and starts nothing
4. [api-trace] state: #21's thread was already running when the halt came; expected: after resume `aayu mission next 889` lists #21 as running and never starts it again
5. [api-trace] state: the ledger cannot be written during `aayu halt`; expected: AAYUDHA_HALT is still set, and the report says `mission claims not released: <reason>; they expire within 15 minutes`

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Claim two stories in two missions, halt, resume, and start again; try a build with the old claim; check a story that was already running; halt with the ledger unwritable.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, and a fake clock; never production

### Technical Owner

engineering-manager

### Links

depends-on #1005

### Assumptions

#### Notes

Only mission claims are in scope; the build-ready loop's claims (`LoopClaimsPath`) do not change. A release line is the existing `loopClaim` with `Release` and the claim's token (`liveClaims`, buildnext.go:352-354). Halt stays the emergency stop for a tool incident (Decision #915, M7): this changes what it releases, not who may run it. Running stories are not released; `aayu mission stop` is for those (#1002).

#### Review guidance

- Check: Criterion 3: the old claim token is refused after the release, not only hidden from the list.
- Check: Criterion 5: a ledger failure never blocks the halt itself.
- Risk: releasing the claim of a story whose thread started between the list and the halt; criterion 4 covers it.
- Ignore: the wording of the halt report outside these lines.

#### Behaviours

1. says: A halt releases every mission claim that has no running thread; rule: limit; source: criteria 1, 3
2. says: A halt never fails because a claim could not be released; rule: never; source: criteria 5
3. says: A story with a running thread is never started twice after a resume; rule: never; source: criteria 4
````
