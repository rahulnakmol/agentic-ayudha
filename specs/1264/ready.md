# rahulnakmol/agentic-ayudha#1264: A delegated build reports a state the ledger did not take when the ledger write fails

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `c3178564d4c4739cc23f21c748b95ad89533fb66c3a8f5ac11f036b4b32ef32e` |
| Completion Goal hash | `592a7775fcc5b3a41521c18513aec2c81e3fd4c5174fd54689d14207edab1c03` |
| Technical Owner | ai-native-engineer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1264/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a host hand-off of #20 waits, and the ledger write of `aayu build 20 --delegate amp --thread <T-id>` fails for a reason other than a moved tip; expected: build exits 5, its output and `--json` result show the waiting hand-off with no task, and stdout does not say `handed-off`
2. [api-trace] state: the ledger write succeeds and the state comment fails; expected: build exits 5, and its output shows the recorded thread, because the ledger holds it
3. [api-trace] state: any other step of `delegateStep` or `reviewLoop` whose `saveDelegation` ledger write fails; expected: build reports the state the ledger holds, and no caller drops the error without saying so
4. [api-trace] state: `delegate/<n>/state.json` holds a record with a type error after a usable `receiver`; expected: `aayu inspect` and `aayu counsel` treat it as no delegation record

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

A delegated build can report a delegation state that the ledger does not hold. In `delegateStep` and `reviewLoop` (internal/cli/delegate.go), each step fills the build result from the new state (`report()` or `d.fill`), then calls `saveDelegation`. `saveDelegation` writes the ledger first, then the state comment. When the ledger write fails for any reason other than a moved tip, build still prints the new status, such as `handed-off` with the thread URL, and its sync step. Some callers also drop the error with `_ = e.saveDelegation(ctx, d)`. PR #1258 maps only `ledger.ErrMoved` in `recordThread` to a refusal that reports the waiting hand-off. The Goldfish review of PR #1258 (round 1) raised this as a P2.

The same class is in two readers of the delegation record. `inspect.go:431` and `counsel.go:441` decode `delegate/<n>/state.json` with `_ = json.Unmarshal`. A record with a type error keeps the fields decoded before the error, such as `receiver`. Inspect then picks its delegated mode, and inspect and counsel name a builder family, from a record that does not parse. PR #1258 fixes the same partial decode in `mission show`.

### Steps to reproduce

1. Hand off story #20 to Amp from a thread whose amp CLI is not signed in, so a host hand-off waits.
2. Make the ledger push fail for a reason other than a moved tip, for example a push that the remote refuses.
3. Run `aayu build 20 --delegate amp --thread <T-id>`.
4. Separately, write `delegate/20/state.json` on the ledger as `{"receiver":"codex","story":"20"}` and run `aayu inspect` on the story's PR.

### Expected behaviour

A build whose ledger write failed reports the state that the ledger holds, exits with an error, and names no new status. A delegation record that does not parse counts as no record.

### Actual behaviour

Build prints `handed-off`, the thread URL, and a sync step, although nothing was recorded. Inspect and counsel read `receiver` from a record that does not parse.

### Completion Goal

1. [api-trace] state: a host hand-off of #20 waits, and the ledger write of `aayu build 20 --delegate amp --thread <T-id>` fails for a reason other than a moved tip; expected: build exits 5, its output and `--json` result show the waiting hand-off with no task, and stdout does not say `handed-off`
2. [api-trace] state: the ledger write succeeds and the state comment fails; expected: build exits 5, and its output shows the recorded thread, because the ledger holds it
3. [api-trace] state: any other step of `delegateStep` or `reviewLoop` whose `saveDelegation` ledger write fails; expected: build reports the state the ledger holds, and no caller drops the error without saying so
4. [api-trace] state: `delegate/<n>/state.json` holds a record with a type error after a usable `receiver`; expected: `aayu inspect` and `aayu counsel` treat it as no delegation record

### Measure

the exit code, text, and `--json` output of `aayu build --delegate`, and inspect's mode and builder family, from unit tests in internal/cli

### Threshold

all 4 criteria hold in `go test ./internal/cli/`

### Scenario

A Kanha records a thread or syncs a delegated story while the ledger push fails, and inspect reads a damaged delegation record.

### Environment

aayu at dev 66397d3; rahulnakmol/agentic-ayudha

### Technical Owner

ai-native-engineer

### Checkpoints

1. A failed ledger write reports the state on the ledger, in every `saveDelegation` caller (criteria 1, 2, 3)
2. A delegation record that does not parse counts as none for inspect and counsel (criterion 4)

### Assumptions

#### Notes

Builds on Bug #1208 (PR #1209) and its follow-up PR #1258. Proposed fix: `saveDelegation` returns whether the ledger write landed. On a failed ledger write, each caller restores the state it read and fills the result from it. A comment failure after the ledger write keeps the new state. In inspect.go and counsel.go, clear the record when `json.Unmarshal` fails, as PR #1258 does in `missionShow`.

#### Review guidance

- Check: every `saveDelegation` caller in `delegateStep` and `reviewLoop`, including the ones that drop the error.
- Ignore: `ledger.ErrMoved` in `recordThread`; PR #1258 covers it.

#### Behaviours

1. says: build reports only the delegation state that the ledger holds; rule: must; source: criteria 1, 2, 3
2. says: a delegation record that does not parse counts as no record; rule: must; source: criterion 4

### Provenance

Request: "file the Bug through aayu (not gh): a delegation ledger write that fails for any reason other than a moved tip still prints handed-off, in every saveDelegation caller in delegateStep. Name the siblings in the same Bug (the partial decode at inspect.go:431 and counsel.go:441), so one fix covers the class."
1. stated (request): "a delegation ledger write that fails for any reason other than a moved tip still prints handed-off"
2. default (scope): "a state comment that fails after the ledger write keeps the new state, because the ledger holds it"
3. stated (request): "in every saveDelegation caller in delegateStep"
4. stated (request): "Name the siblings in the same Bug (the partial decode at inspect.go:431 and counsel.go:441), so one fix covers the class"
````
