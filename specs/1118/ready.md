# rahulnakmol/agentic-ayudha#1118: A send to the Kanha has no time limit, so a stalled receiver CLI makes ship or build hang

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `c93d36f57d1062608d76b9c651eb6be5fd611df203f096512de683a099cb0cee` |
| Completion Goal hash | `cea919f34d6d9d9ddbc00c7ccc035f32360e1f0b5a90eaf3d30026f2076036fd` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1118/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu ship 41` merges story #40 of #889, and the Kanha's `amp threads continue` never returns; expected: ship exits 0 within the send limit with the merge, the Complete section, and status:merged in place, and prints `next: the Kanha of #889 was not reached: amp: context deadline exceeded; the safety tick wakes it`
2. [api-trace] state: `aayu build 35 --stop "spec unclear"` on story #35 of #889, and the Kanha's `amp threads continue` never returns; expected: build exits 4 within the send limit, #35 is status:blocked, and delegate/35/state.json has the event `blocked #35: spec unclear: run aayu mission next 889 (not sent: …)`

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

A send to the Kanha has no time limit, so a stalled receiver CLI makes ship or build hang after its work is done. `tellKanha` (internal/cli/kanha.go:113) sends through `Adapter.FollowUp` (kanha.go:132) with the command's context. That context ends only on Ctrl-C (cmd/aayudha/main.go:24, `signal.NotifyContext`). Two callers wait on it: ship's `tellMerged` after a merge (internal/cli/ship.go:659, Story #1007), and build's `markBlocked` and `failedStart` (internal/cli/build.go:1155 and :1187, Story #1008). CodeRabbit found it on PR 1098 (internal/cli/ship.go:625).

### Steps to reproduce

1. Start a mission of #889 on amp, with its Kanha an Amp thread.
2. Make `amp threads continue` never return (a stub that sleeps, or a hung network).
3. Run `aayu ship <pr>` on a story PR of #889 that ship merges, or `aayu build <n> --stop "spec unclear"` on a story of #889.
4. The command does not exit, and prints nothing more, until Ctrl-C.

### Expected behaviour

A send to the Kanha ends after a fixed time, 2 minutes. A send that runs past it is a send that failed: ship prints its `next: the Kanha of #<f> was not reached: <reason>; the safety tick wakes it` line and exits 0 with the merge in place. build records the event as not sent and goes on.

### Actual behaviour

The send waits for the receiver CLI as long as it runs. ship has already merged, written the ship record, released the lease, and checkpointed, but it does not print its result or exit. build does not save its state or exit.

### Completion Goal

1. [api-trace] state: `aayu ship 41` merges story #40 of #889, and the Kanha's `amp threads continue` never returns; expected: ship exits 0 within the send limit with the merge, the Complete section, and status:merged in place, and prints `next: the Kanha of #889 was not reached: amp: context deadline exceeded; the safety tick wakes it`
2. [api-trace] state: `aayu build 35 --stop "spec unclear"` on story #35 of #889, and the Kanha's `amp threads continue` never returns; expected: build exits 4 within the send limit, #35 is status:blocked, and delegate/35/state.json has the event `blocked #35: spec unclear: run aayu mission next 889 (not sent: …)`

### Measure

Completion Goal criteria observed

### Threshold

all 2 criteria

### Scenario

A Kanha's Amp thread hangs while ship merges a story and while a builder stops one.

### Environment

go test of internal/cli with the ship world and the mission world, a receiver stub that waits until its context ends, and a shortened send limit; never production

### Technical Owner

engineering-manager

### Assumptions

#### Notes

The fix bounds only the send in `tellKanha`, so every caller gets it. The event write to the ledger keeps the command's context. It changes no record field and no message.

#### Review guidance

- Check: Criteria 1 and 2: the stub waits until its context ends, so both tests hang on the old code.
- Check: the ledger write after the send still uses the command's context.
- Ignore: the exact limit, 2 minutes.
- Risk: a slow but healthy receiver past the limit is told nothing; the safety tick wakes it.

#### Behaviours

1. says: A send to the Kanha never holds ship or build past its limit; rule: never; source: criteria 1, 2
````
