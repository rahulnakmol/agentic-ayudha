# rahulnakmol/agentic-ayudha#1162: ship and proof land can take the pack and the gate checks from two gate runs

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `ec9074c382c6e60fa8f894787c722689bfcd3d1380c3f5934484ad433145f1cf` |
| Completion Goal hash | `b1a8e703893e793f553d9795dd12bf30bf8a57c7e0872cfab08ac3c7fc37e0b8` |
| Technical Owner | quality-architect |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1162/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: two gate runs of one pull request on its head, the newer one failed in a job outside the gate checks, the older one green with a pack; expected: ship and proof land require the gate checks of the run whose pack they land to be green, and refuse when they are not
2. [api-trace] state: the newest gate run of the pull request has a red gate check; expected: ship and proof land refuse with "the merge gate must be green", as before

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

ship and proof land take the pack and the gate checks from different gate runs. `gatePack` (internal/cli/ship.go) takes the pack from the newest gate run of the pull request that completed with success. `gateGreen` reads the newest run of each gate check from any of the pull request's gate suites. A newer gate run can fail in a job outside the three gate checks, for example `aayudha/inspect-reply`. Then `gatePack` takes the older run's pack, and `gateGreen` reads the newer run's green checks. The pack and the checks it lands with then come from two runs. Both runs are the pull request's own runs on the same head, and the older run is green. So no pack of another pull request or another head can land. CodeRabbit raised this on PR #1160 at af4b4ce as a Minor finding (internal/cli/ship.go:680).

### Steps to reproduce

1. Open a pull request with a story and let the merge gate run on its head H. The run succeeds and uploads its pack.
2. Start a second gate run on H, for example with an edit of the body. Its gate checks succeed, but another job of the run fails.
3. Run `aayu ship <pr>`, or `aayu proof land <pr>` after the merge.

### Expected behaviour

The gate checks that ship and proof land read are the checks of the run whose pack they land. A newer red gate check in any of the pull request's suites still blocks.

### Actual behaviour

`gatePack` takes the pack of the first run, and `gateGreen` accepts the gate checks of the second run.

### Completion Goal

1. [api-trace] state: two gate runs of one pull request on its head, the newer one failed in a job outside the gate checks, the older one green with a pack; expected: ship and proof land require the gate checks of the run whose pack they land to be green, and refuse when they are not
2. [api-trace] state: the newest gate run of the pull request has a red gate check; expected: ship and proof land refuse with "the merge gate must be green", as before

### Measure

the exit code and output of `aayu ship` and `aayu proof land`, from their unit tests

### Threshold

both criteria hold in `go test ./internal/cli/ -run 'TestShipOnProof|TestProofLand'`

### Scenario

A person or an agent runs `aayu ship` or `aayu proof land` on a head that has two gate runs.

### Environment

aayu built from dev after PR #1160; rahulnakmol/agentic-ayudha

### Technical Owner

quality-architect

### Checkpoints

1. The gate checks of the run whose pack lands must be green, and every newer gate check of the pull request still counts (criteria 1, 2)

### Assumptions

#### Notes

CodeRabbit suggests passing the pack run's check suite to `gateGreen` and checking that suite's gate checks after the scan across all suites. `internal/cli/ship.go` is an `always_person` path, so a person ships the fix.

#### Review guidance

- Check: a pack never lands with gate checks from another run.
- Ignore: which runs count as the pull request's own; #1160 and Bug #1161 own that rule.

#### Behaviours

1. says: The pack and the gate checks that land together come from one gate run; rule: must; source: criteria 1, 2

### Provenance

Request: "the pack and the checks come from one run"
1. stated (request): "the pack and the checks come from one run"
2. default (scope): "a newer red gate check still blocks"
````
