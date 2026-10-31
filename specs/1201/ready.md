# rahulnakmol/agentic-ayudha#1201: A Decision superseded through an accepted successor's chain stays open and asks to be accepted

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `8bf9d62b56b30d64ea64e4fb0b96a2ebcaeec8c9aa60a41213f1b01f4872121c` |
| Completion Goal hash | `99a8ee44913368d2e7177abcc6b2073d03739ecb8de5061842dbe532a9f78366` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1201/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: #2 supersedes #1, #3 supersedes #2, and a person accepts #3; expected: #1 and #2 are both closed with the superseded marker naming #3
2. [api-trace] state: #1 is open and its only successor #2 is closed as superseded by accepted #3; expected: `aayu inbox --list` shows no Decision card for #1, and a housekeeping card offers to close it as superseded by #3
3. [api-trace] state: #1 is open and its successor #2 is open, not accepted; expected: #1's card stays, as today
4. [api-trace] state: the chain has a cycle (#1 supersedes #2, #2 supersedes #1); expected: nothing is closed, and the card names the cycle

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

An open Decision whose successor was itself superseded stays open, and shows in the morning inbox as a one-way door to accept. Decision #863 (build by waves with `in_flight`) is superseded by #868 (`supersedes #863`), and #868 by #918, which @rahulnakmol accepted on 2026-10-23. #868 is closed; #863 is still open with status:planned, has a lapsed-SLA escalation, and was the first card of `aayu inbox --list` on 2026-10-31, with "Accept" as its recommended action. Accepting #918 marks only the Decision it names as superseded (`SupersededMarker`, internal/decide/decide.go:26), not that Decision's own predecessors.

### Steps to reproduce

1. Decision B has `supersedes #A`; Decision C has `supersedes #B`.
2. A person accepts C. B closes as superseded; A was never closed.
3. Run `aayu inbox --list`.

### Expected behaviour

A is closed as superseded by C, and no inbox card offers to accept it.

### Actual behaviour

A stays open, and the inbox offers to accept it as a one-way door.

### Completion Goal

1. [api-trace] state: #2 supersedes #1, #3 supersedes #2, and a person accepts #3; expected: #1 and #2 are both closed with the superseded marker naming #3
2. [api-trace] state: #1 is open and its only successor #2 is closed as superseded by accepted #3; expected: `aayu inbox --list` shows no Decision card for #1, and a housekeeping card offers to close it as superseded by #3
3. [api-trace] state: #1 is open and its successor #2 is open, not accepted; expected: #1's card stays, as today
4. [api-trace] state: the chain has a cycle (#1 supersedes #2, #2 supersedes #1); expected: nothing is closed, and the card names the cycle

### Measure

Completion Goal criteria observed

### Threshold

all 4 criteria

### Scenario

Accept the last Decision of a three-step supersede chain; list the inbox with the old head still open; repeat with an unaccepted successor and a cycle.

### Environment

go test of internal/cli with a fake GitHub and seeded Decisions; never production

### Checkpoints

1. Accepting a Decision closes every Decision in its supersede chain (criteria 1)
2. The inbox shows a stale chain head as housekeeping, not a Decision (criteria 2, 3, 4)

### Technical Owner

engineering-manager

### Assumptions

#### Notes

#863's state on 2026-10-31 is the case: #868 supersedes #863; #918 supersedes #868 and is accepted. Closing as superseded writes nothing to the accepted Decision. The housekeeping card's close is the same write as today's supersede.

#### Review guidance

- Check: criterion 3: an unaccepted successor never closes its predecessor.
- Check: criterion 4: a cycle closes nothing.
- Ignore: the wording of the card.
- Risk: closing a Decision a person still means to answer; only an accepted successor closes it.

#### Behaviours

1. says: A Decision replaced through an accepted successor never asks a person to accept it; rule: never; source: criteria 1, 2
2. says: Only an accepted successor closes a predecessor; rule: never; source: criteria 3, 4

### Provenance

Request: "we indee dwan this [build] agents and mission milestones"
1. stated (request): the accepted choice (#918) stands, and the old one (#863) goes
2. stated (request): the morning inbox shows only real decisions
3. default (scope): an open successor does not close its predecessor
4. default (scope): a cycle closes nothing
````
