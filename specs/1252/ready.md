# rahulnakmol/agentic-ayudha#1252: aayu ready --dry-run for Approve to build starts the device flow and waits for a person

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `ea27b9a34e4ccd3d16fbb4c2f110cd620144df0788bd63f33f25e2dca52402ee` |
| Completion Goal hash | `8aeb325ad0633739a1c5170618a5f822f12ea88d60409c8563d9e9c133c07639` |
| Technical Owner | ai-native-engineer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1252/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu ready <f> --dry-run --milestones <valid>` for a Feature that needs Approve to build; expected: the approval lines and the plan are printed, exit 0, no device-flow request reaches the fake GitHub, and the ledger has no write
2. [api-trace] state: the same with an invalid milestones string; expected: the same refusal as without `--dry-run`, and no device-flow request
3. [api-trace] state: `--dry-run` of `mission start`, `mission budget --to`, and `mission extend`; expected: each prints its effect, sends no device-flow request, and writes nothing

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

`aayu ready <f> --dry-run` for Approve to build starts GitHub's device flow, so a dry run asks a person for a code and waits. `approveRun` builds the report with `DryRun: c.DryRun()` but calls `e.presenceCheck` before it reads that flag (internal/cli/readyapprove.go:111 and :119). On 2026-11-02, the Kanha of #1170 ran `aayu ready 1170 --dry-run --milestones ...` to check a milestones string. The command printed a device code and waited 10 minutes, until the Kanha stopped it. The plan itself was valid. The same order may apply to the other human-only commands that call `presenceCheck` (internal/cli/mission.go:928, missiongate.go:1566, missiontime.go:871); that is unverified.

### Steps to reproduce

1. Feature #1170 has a spec baseline that is newer than its Approve to build.
2. In a shell with no `AAYU_AGENT=1`, run `aayu ready 1170 --dry-run --milestones '<valid string>'`.

### Expected behaviour

The dry run prints what Approve to build would approve, the mission plan, and the estimate, and exits 0. It starts no device flow and writes nothing.

### Actual behaviour

The dry run prints the same lines, then a device code, and waits for a person.

### Completion Goal

1. [api-trace] state: `aayu ready <f> --dry-run --milestones <valid>` for a Feature that needs Approve to build; expected: the approval lines and the plan are printed, exit 0, no device-flow request reaches the fake GitHub, and the ledger has no write
2. [api-trace] state: the same with an invalid milestones string; expected: the same refusal as without `--dry-run`, and no device-flow request
3. [api-trace] state: `--dry-run` of `mission start`, `mission budget --to`, and `mission extend`; expected: each prints its effect, sends no device-flow request, and writes nothing

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Run each human-only command with `--dry-run` against a fake GitHub that records device-flow requests.

### Environment

go test of internal/cli with a fake GitHub and a fake ledger; never production

### Checkpoints

1. Approve to build dry run ends before the presence check (criteria 1, 2)
2. The mission's human-only verbs do the same (criteria 3)

### Technical Owner

ai-native-engineer

### Assumptions

#### Notes

A dry run must never ask for presence: it writes nothing, so there is nothing to approve. This is friction for agents that check a command before they give it to the founder.

#### Review guidance

- Check: the presence check is never reached when `DryRun()` is true, and a real run still reaches it.
- Ignore: the wording of the dry-run lines.
- Risk: a dry run that skips a validation the real run does; criterion 2 catches it.

#### Behaviours

1. says: A dry run never starts the device flow; rule: never; source: criteria 1, 3

### Provenance

Request: "we are really loosing velocity. see what we can do such a way we ship everyday substantial ones with no human dependency"
1. default (scope): Approve to build dry run
2. default (scope): invalid input still refused
3. default (scope): the mission's human-only verbs
````
