# rahulnakmol/agentic-ayudha#1206: ship asks for the device code before checks that need no person, so a refused ship wastes the code

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `e69365adce53c4054469994bfc1a88fac78c2aff1c1d6939ab81cdc40e451698` |
| Completion Goal hash | `925fb9bfbf9c2af62b934362c1f3e1249eb3bf04bcf5b36adb47b05b1c94d3f7` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1206/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: PR #20's story has no plan-time review for its never layer; expected: `aayu ship 20 --solo-bypass` refuses with the plan review's next command and asks for no device code
2. [api-trace] state: PR #20 has no "Solo-mode bypass:" comment that names its head; expected: ship refuses before the device flow and names the comment to post
3. [api-trace] state: every precondition holds; expected: ship asks for the device code once and merges
4. [api-trace] state: an agent lane; expected: ship refuses as today

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

`aayu ship <pr> --solo-bypass` asks for the person's device code before it checks what it can check without them, so a code is spent on a ship that refuses. On 2026-10-31 at 14:11 UTC, @rahulnakmol entered a code for `aayu ship 1194 --solo-bypass`. Ship then refused: the footprint touches never layer L07, so #1193 needs a plan-time review (`aayu plan 1193 --solo-bypass`), and #1193 had no plan on the ledger. `requireHuman` runs first (`internal/cli/ship.go:124-125`), and the plan-time review check comes later (`internal/cli/ship.go:291`).

### Steps to reproduce

1. PR #20 refs Story #10, touches a never layer, and #10 has no plan-time review or bypass.
2. Run `aayu ship 20 --solo-bypass`.

### Expected behaviour

Ship refuses before the device flow, naming the missing plan review and its next command.

### Actual behaviour

Ship asks for the device code, the person enters it, and then ship refuses.

### Completion Goal

1. [api-trace] state: PR #20's story has no plan-time review for its never layer; expected: `aayu ship 20 --solo-bypass` refuses with the plan review's next command and asks for no device code
2. [api-trace] state: PR #20 has no "Solo-mode bypass:" comment that names its head; expected: ship refuses before the device flow and names the comment to post
3. [api-trace] state: every precondition holds; expected: ship asks for the device code once and merges
4. [api-trace] state: an agent lane; expected: ship refuses as today

### Measure

Completion Goal criteria observed

### Threshold

all 4 criteria

### Scenario

Ship a solo-bypass PR with no plan review, with no bypass comment, and with everything in place; then from an agent lane.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, and a fake device flow; never production

### Checkpoints

1. Every check that needs no person runs before the device flow (criteria 1, 2)
2. A ship that passes them asks for one code (criteria 3, 4)

### Technical Owner

engineering-manager

### Assumptions

#### Notes

The checks read GitHub and the ledger only; the device flow still comes before any write. ship.go is in the narrow set, so the PR waits for a person's look.

#### Review guidance

- Check: no write happens before the device flow.
- Check: criterion 3: still exactly one code.
- Ignore: the wording of the refusals.
- Risk: a check that passes before the code and fails after it; the head is read again after the code, as today.

#### Behaviours

1. says: A person's code is never asked for a ship that is known to refuse; rule: never; source: criteria 1, 2
2. says: A ship that can merge asks for one code; rule: limit; source: criteria 3

### Provenance

Request: "I can only check in every 4 hrs so we need to ensure we can move the journey ahead"
1. stated (request): no wasted check-in steps
2. default (scope): the same for the bypass comment
3. default (scope): one code for a ship that merges
4. default (scope): agent lanes refused as today
````
