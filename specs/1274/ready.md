# rahulnakmol/agentic-ayudha#1274: Every other human-only verb still starts the device flow in a dry run

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `4282ec77144ef2bed04bbe861ebde1896e68b61272408fbf3e4c9f6bc1605669` |
| Completion Goal hash | `2b7bcff1c1929cee3148e021ce39c17a5c3118cdf74e597e10039f144921f5e4` |
| Technical Owner | ai-native-engineer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1274/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `--dry-run` of each listed verb; expected: it prints its effect, sends no device-flow request to the fake GitHub, writes nothing, and exits 0
2. [api-trace] state: a test walks every `presenceCheck` caller; expected: each has a dry-run case, so a new human-only verb without one fails the test
3. [api-trace] state: `aayu ready <n> --dry-run`; expected: no bot token is minted

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

After Bug #1252 (PR #1262), `--dry-run` of Approve to build, `mission start`, `mission budget --to`, and `mission extend` asks for no device code. The other human-only verbs still start the device flow in a dry run. The #1252 builder lists `mission budget --keep`, `mission stop` and `resume`, `mission recover` and `retry`, `mission gate --once-more`, `ready --part`, `ready --early-build`, `scope --accept`, and `inspect --approve`, each a `presenceCheck` caller (internal/cli/presence.go:136; callers in internal/cli/missionstop.go, missionrecover.go, missiongate.go, ready.go, ready_early.go, scope.go, inspect.go). The builder also reports that `aayu ready` mints a bot token before it reads the issue type, in a dry run too; that is unverified.

### Steps to reproduce

1. In a shell with no `AAYU_AGENT=1`, run any listed verb with `--dry-run`.

### Expected behaviour

Each dry run prints its effect, exits 0, sends no device-flow request, and writes nothing.

### Actual behaviour

Each dry run prints a device code and waits for a person.

### Completion Goal

1. [api-trace] state: `--dry-run` of each listed verb; expected: it prints its effect, sends no device-flow request to the fake GitHub, writes nothing, and exits 0
2. [api-trace] state: a test walks every `presenceCheck` caller; expected: each has a dry-run case, so a new human-only verb without one fails the test
3. [api-trace] state: `aayu ready <n> --dry-run`; expected: no bot token is minted

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Run every human-only verb with `--dry-run` against a fake GitHub that records device-flow and token requests.

### Environment

go test of internal/cli with a fake GitHub and a fake ledger; never production

### Checkpoints

1. Every listed verb's dry run asks for no code (criteria 1)
2. A test keeps new verbs honest (criteria 2)
3. Ready's dry run mints no token (criteria 3)

### Technical Owner

ai-native-engineer

### Assumptions

#### Notes

Follows #1252. The fix there likely gives the shape: end the dry run before `presenceCheck`, after the same validation the real run does.

#### Review guidance

- Check: criterion 2: the test finds callers from the code, not from a hand list.
- Ignore: the wording of the dry-run lines.
- Risk: a dry run that skips a validation the real run does.

#### Behaviours

1. says: A dry run never starts the device flow; rule: never; source: criteria 1, 2

### Provenance

Request: "we are really loosing velocity. see what we can do such a way we ship everyday substantial ones with no human dependency"
1. default (scope): every human-only verb's dry run
2. default (scope): a test over every presence caller
3. default (scope): no token minted in a dry run
````
