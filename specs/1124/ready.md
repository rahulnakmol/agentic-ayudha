# rahulnakmol/agentic-ayudha#1124: A hand-off or follow-up to a receiver has no time limit, so a stalled receiver CLI makes build or mission start hang

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `aa803d7366aa042f1ed494cfc35f2289044ef019cee1ef7d949cdec9fec3f127` |
| Completion Goal hash | `85d295f45051b4931c21f06300c4efdcc28d1783b700de870fac34c28a81094f` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1124/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu build 50 --delegate amp` syncs story #50, inspect asks for fixes, and `amp threads continue` never returns; expected: build exits 5 within the time limit with `follow-up to amp: amp: context deadline exceeded`
2. [api-trace] state: `aayu build 50 --delegate amp` hands off story #50, and `amp -ox` never returns; expected: build exits 5 within the time limit, and delegate/50/state.json has the event `hand-off failed: amp: context deadline exceeded`
3. [api-trace] state: `aayu mission start 889` on amp starts a new Kanha, and `amp -ox` never returns; expected: Start exits within the time limit, says `amp: context deadline exceeded`, and specs/889/approved.json has no parent

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

A hand-off or a follow-up to a receiver has no time limit, so a stalled receiver CLI makes `aayu build` or `aayu mission start` hang. Bug #1118 put a time limit only on the send to the Kanha in `tellKanha` (`kanhaSendTimeout`). These calls still use the command's context, which ends only on Ctrl-C (cmd/aayudha/main.go:24):

- build's follow-ups to a story's receiver: the failed-checks follow-up (internal/cli/delegate.go:731) and the findings follow-up (delegate.go:807);
- build's hand-off of a story (delegate.go:511);
- `startKanha`'s brief to the drafting Amp thread (internal/cli/kanha.go:69) and its hand-off of a new Kanha (kanha.go:73).

CodeRabbit found it on pull request 1119 (delegate.go:736-746, outside that diff).

### Steps to reproduce

1. Delegate story #50 of #889 to amp, with a PR that inspect asks to fix.
2. Make `amp threads continue` never return (a stub that sleeps, or a hung network).
3. Run `aayu build 50 --delegate amp` to sync.
4. The command does not exit until Ctrl-C. The same happens with `amp -ox` on a first hand-off, and with `aayu mission start 889` on amp.

### Expected behaviour

Each hand-off and follow-up to a receiver ends after the same fixed time as a send to the Kanha, 2 minutes. A call past it is a call that failed, and it says `<tool>: context deadline exceeded`. The command then fails or records the failure as it does for any other receiver error.

### Actual behaviour

The call waits for the receiver CLI as long as it runs. build does not save its state or exit, and Start does not record or report the Kanha.

### Completion Goal

1. [api-trace] state: `aayu build 50 --delegate amp` syncs story #50, inspect asks for fixes, and `amp threads continue` never returns; expected: build exits 5 within the time limit with `follow-up to amp: amp: context deadline exceeded`
2. [api-trace] state: `aayu build 50 --delegate amp` hands off story #50, and `amp -ox` never returns; expected: build exits 5 within the time limit, and delegate/50/state.json has the event `hand-off failed: amp: context deadline exceeded`
3. [api-trace] state: `aayu mission start 889` on amp starts a new Kanha, and `amp -ox` never returns; expected: Start exits within the time limit, says `amp: context deadline exceeded`, and specs/889/approved.json has no parent

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

A receiver's Amp CLI hangs while build forwards fixes, while build hands off a story, and while Start starts the Kanha.

### Environment

go test of internal/cli with the delegate and mission worlds, a receiver stub that waits until its context ends and then returns a killed process's error, and a shortened time limit; never production

### Technical Owner

engineering-manager

### Assumptions

#### Notes

The fix reuses the pattern of Bug #1118: a context with the time limit around each receiver call, the tool name and `ctx.Err()` as the error when the limit ended it, and `receiverWaitDelay` for a child that keeps the output open. Ledger writes keep the command's context.

#### Review guidance

- Check: each criterion's stub waits until its context ends, so each test hangs on the old code.
- Check: the ledger writes after each call still use the command's context.
- Ignore: the exact limit, 2 minutes.
- Risk: a hand-off that takes more than 2 minutes in a healthy receiver now fails; the receiver's own record shows it.

#### Behaviours

1. says: A hand-off or follow-up to a receiver never holds build or Start past its time limit; rule: never; source: criteria 1, 2, 3
````
