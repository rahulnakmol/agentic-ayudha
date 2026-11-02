# rahulnakmol/agentic-ayudha#1267: Approve to build again resets a milestone gate that already passed, so every later PR waits for that gate again

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `6de812631da05f8748b10014353d3b28fa4b0ba0225ba078cba7ddfb4cfa8d41` |
| Completion Goal hash | `f61ed44bb74d32dc2f2dc62a4c3a7fceb640b0f8c9ea7d5847eb7430a82bd429` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1267/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: Approve to build again, where MS1 has the same name and stories and its gate passed; expected: the plan keeps MS1's gate passed, with the same commit, time, and gate fixes, and `aayu ship` of an MS2 PR is not held by MS1
2. [api-trace] state: Approve to build again, where MS1 gains or loses a story; expected: MS1's gate waits, and the approval lines say that MS1's gate runs again and why
3. [api-trace] state: Approve to build again with no `--milestones`; expected: every gate is kept, as today

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

Approve to build again with `--milestones` resets a milestone gate that has already passed. `approveRun.missionPlan` builds the milestones from `parseMilestones` (internal/cli/mission.go:351), and that function gives every milestone a new gate in the waits state (internal/cli/mission.go:647). It never reads the old plan's gate for a milestone with the same name and the same stories. On 2026-11-02, MS1 of #1170 passed its gate on dev b83e187 at 04:05Z, on its third try. That evening @rahulnakmol ran Approve to build again, with the same MS1 stories, because #1250, #1253, #1254 and #1256 had joined the mission. After that, missions/1170/plan.json had MS1's gate at "waits". `aayu ship 1261` (#1250, MS2) then refused with "#1250 is in MS2: it merges after MS1's gate passes". So every MS2 and BUGS PR waits again for MS1's eval gate, which took three tries the first time.

### Steps to reproduce

1. A mission's MS1 gate passed, and MS2 has a proven PR.
2. Run `aayu ready <f> --milestones '<the same MS1 stories> MS2=...'` (Approve to build again).
3. Run `aayu mission show <f>` and `aayu ship <MS2 PR>`.

### Expected behaviour

A milestone that keeps its name and its stories keeps its gate: passed stays passed, with its dev commit and time. Its gate fixes stay too. A milestone whose stories changed gets a new gate that waits.

### Actual behaviour

Every milestone's gate is reset to waits, so a passed MS1 holds every later PR again.

### Completion Goal

1. [api-trace] state: Approve to build again, where MS1 has the same name and stories and its gate passed; expected: the plan keeps MS1's gate passed, with the same commit, time, and gate fixes, and `aayu ship` of an MS2 PR is not held by MS1
2. [api-trace] state: Approve to build again, where MS1 gains or loses a story; expected: MS1's gate waits, and the approval lines say that MS1's gate runs again and why
3. [api-trace] state: Approve to build again with no `--milestones`; expected: every gate is kept, as today

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Approve a mission again after MS1's gate passed: once with the same MS1, once with MS1 changed, and once with no --milestones.

### Environment

go test of internal/cli with a fake GitHub and a fake ledger; never production

### Checkpoints

1. A passed gate survives an Approve to build that keeps its milestone (criteria 1, 3)
2. A changed milestone runs its gate again, and says so (criteria 2)

### Technical Owner

engineering-manager

### Assumptions

#### Notes

The Kanha ran `aayu mission gate 1170` on dev ab8ab07 to pass MS1 again; that run waits for build. This Bug stops a running mission, so it stays in v0.4.0.

#### Review guidance

- Check: criterion 2: a milestone with any changed story never keeps an old pass.
- Ignore: the wording of the approval lines.
- Risk: a pass kept for a milestone whose stories changed; criterion 2 catches it.

#### Behaviours

1. says: Approving again keeps a passed gate for an unchanged milestone; rule: limit; source: criteria 1, 3
2. says: A changed milestone runs its gate again; rule: never; source: criteria 2

### Provenance

Request: "we are really loosing velocity. see what we can do such a way we ship everyday substantial ones with no human dependency"
1. default (scope): keep a passed gate for an unchanged milestone
2. default (scope): a changed milestone runs its gate again
3. default (scope): no --milestones keeps every gate
````
