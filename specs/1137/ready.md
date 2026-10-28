# rahulnakmol/agentic-ayudha#1137: build's sync reads a receiver's task with no time limit, so a stalled receiver makes build hang

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `4463c140abe448df91871464abd187b07aef2df4f8690db39e36b7928551d630` |
| Completion Goal hash | `29a25c415399490034d2dfb01aad6f19ebc659b659aca23cde0d4cb0e71dda38` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1137/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu build 50 --delegate codex` syncs story #50, and `codex cloud status` never returns; expected: build exits 5 within the time limit with `build #50 --delegate codex: status: codex: context deadline exceeded`
2. [api-trace] state: `aayu build 50 --delegate cursor` syncs story #50, and the Cursor API's read of the run never answers; expected: build exits 5 within the time limit with `build #50 --delegate cursor: status: cursor: context deadline exceeded`
3. [api-trace] state: `aayu build 50 --delegate claude` syncs story #50, and `git ls-remote` for its branch never returns; expected: build exits 5 within the time limit with `build #50 --delegate claude: status: claude: context deadline exceeded`

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

build's sync reads a delegated story's task with `Adapter.Status`, and that read has no time limit. A stalled receiver makes `aayu build <n> --delegate <receiver>` hang until Ctrl-C. Bug #1124 put `receiverTimeout` (internal/cli/kanha.go:125) on each hand-off and follow-up, but the sync's Status call still uses the command's context (internal/cli/delegate.go:540). The calls under it:

- Codex: `codex cloud status` and, when the task is done, `codex cloud diff` (internal/delegate/receivers.go:415);
- Cursor: a GET of the agent's run on the Cursor API (receivers.go:155), with `http.DefaultClient` when no client is set (receivers.go:99);
- Claude Code: `git ls-remote --heads origin` for the brief's branch (receivers.go:326, internal/cli/delegate.go:133).

Amp's Status makes no call (receivers.go:234). The local Kanha is out of scope: #1125 adds `[delegate] local_timeout_minutes` for the local start and each resume.

### Steps to reproduce

1. Delegate story #50 of #889 to codex, so the ledger has its task.
2. Make `codex cloud status` never return (a stub that sleeps, or a hung network).
3. Run `aayu build 50 --delegate codex` to sync.
4. The command does not exit until Ctrl-C. The same happens with a Cursor API that never answers, and with a `git ls-remote` that never returns on claude.

### Expected behaviour

The sync's Status call ends after `receiverTimeout`, the same limit as a hand-off or a follow-up. A call past it is a call that failed, and it says `<tool>: context deadline exceeded`. build then exits 5 as it does for any other status error.

### Actual behaviour

The sync waits for the receiver as long as it runs, and build does not exit.

### Completion Goal

1. [api-trace] state: `aayu build 50 --delegate codex` syncs story #50, and `codex cloud status` never returns; expected: build exits 5 within the time limit with `build #50 --delegate codex: status: codex: context deadline exceeded`
2. [api-trace] state: `aayu build 50 --delegate cursor` syncs story #50, and the Cursor API's read of the run never answers; expected: build exits 5 within the time limit with `build #50 --delegate cursor: status: cursor: context deadline exceeded`
3. [api-trace] state: `aayu build 50 --delegate claude` syncs story #50, and `git ls-remote` for its branch never returns; expected: build exits 5 within the time limit with `build #50 --delegate claude: status: claude: context deadline exceeded`

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

A receiver's CLI or API hangs while build syncs a delegated story.

### Environment

go test of internal/cli with the delegate world, a receiver stub that waits until its context ends, and a shortened time limit; never production

### Technical Owner

engineering-manager

### Assumptions

#### Notes

The fix reuses `withReceiverTimeout` (internal/cli/kanha.go:130) around the sync's Status call. The ledger writes keep the command's context.

#### Review guidance

- Check: each criterion's stub waits until its context ends, so each test hangs on the old code.
- Check: Codex's diff fetch after a done status is inside the same limit.
- Ignore: the exact limit, 2 minutes.
- Risk: a healthy receiver that takes more than 2 minutes to answer a status read now fails; build again syncs it.

#### Behaviours

1. says: A sync's status read never holds build past its time limit; rule: never; source: criteria 1, 2, 3

### Checkpoints

1. A codex sync whose status stub never returns exits 5 with the deadline error (criteria 1)
2. A cursor sync and a claude sync whose reads never return exit 5 the same way (criteria 2, 3)

### Provenance

Request: "Adapter.Status (build's sync) has no time limit. Leave LocalKanha out: #1125 adds [delegate] local_timeout_minutes for the local start and each resume."
1. stated (request): "Adapter.Status (build's sync) has no time limit"
2. stated (request): "Adapter.Status (build's sync) has no time limit"
3. stated (request): "Adapter.Status (build's sync) has no time limit"
````
