# rahulnakmol/agentic-ayudha#1163: A person's recorded approval of a head never clears a rung-B verdict whose only gap is unclear criteria

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `540a91f3ab5396dfa2b383c719e70e75763142d6418da4cee898925cdd503727` |
| Completion Goal hash | `512fa95d85d7da0ed04ffdad4daf87084136a03730677b121626aea1da158c91` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1163/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: rung B gives fix with no findings and only unclear criteria, and the [autonomy] approver has an `aayu inspect --approve` for that exact head on the ledger; expected: inspect says merge, and its comment names the approval and which criteria the person accepted
2. [api-trace] state: rung B gives fix with a finding of any severity, with or without an approval of the head; expected: inspect still says fix
3. [api-trace] state: rung B gives fix with no findings and only unclear criteria, and the only approval on the ledger is for another head; expected: inspect still says fix
4. [api-trace] state: rung B gives fix with no findings and only unclear criteria, and no approval is on the ledger; expected: inspect says fix, as today

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

A person's recorded approval of a head never clears a rung-B verdict whose only gap is criteria it marked unclear. Rung B ([inspect] paid_reviewer) gives `fix` when it cannot see a criterion's evidence in the pack, even with no finding. A tier-2 eval result is such a criterion: it cannot run on a PR head, because the ai-gateway environment allows only dev, rolling, and main. The [autonomy] approver can record `aayu inspect <pr> --approve` for the head, but the model path of inspect (internal/cli/inspect.go, runInspect) never reads it. Only the human rung of the app path reads approvals. So the PR stays at `fix`, and every push spends a review round. PR #1146 (Bug #1132) is held at 734b107 this way: rung B gave `fix` with no finding and criteria 1 and 2 unclear, and @rahulnakmol's approval of 734b107 is on the ledger.

### Steps to reproduce

1. Open a Story or Bug PR whose criteria need evidence that the pack cannot hold, such as a tier-2 eval result.
2. Let the merge gate's rung B review the head. It gives `fix` with no finding and marks those criteria unclear.
3. As the [autonomy] approver, run `aayu inspect <pr> --approve --reason "…"` for that head.
4. Run the merge gate's inspect check again at the same head.

### Expected behaviour

inspect says `merge`, and its comment names the approval and the criteria the person accepted.

### Actual behaviour

inspect still says `fix`. The approval has no effect on a rung-B verdict.

### Completion Goal

1. [api-trace] state: rung B gives fix with no findings and only unclear criteria, and the [autonomy] approver has an `aayu inspect --approve` for that exact head on the ledger; expected: inspect says merge, and its comment names the approval and which criteria the person accepted
2. [api-trace] state: rung B gives fix with a finding of any severity, with or without an approval of the head; expected: inspect still says fix
3. [api-trace] state: rung B gives fix with no findings and only unclear criteria, and the only approval on the ledger is for another head; expected: inspect still says fix
4. [api-trace] state: rung B gives fix with no findings and only unclear criteria, and no approval is on the ledger; expected: inspect says fix, as today

### Measure

the disposition and the comment of `aayu inspect`, from the unit tests of inspect

### Threshold

all four criteria hold in `go test ./internal/cli/ -run TestInspect`

### Scenario

The [autonomy] approver approves a head whose criteria only a person or a later run can show, and the merge gate judges it again.

### Environment

aayu built from dev 4e9bd37; rahulnakmol/agentic-ayudha

### Technical Owner

engineering-manager

### Checkpoints

1. A recorded approval of the exact head lifts a rung-B fix to merge only when the review has no finding and no unmet criterion, and the floors still hold (criteria 1, 2, 3, 4)

### Assumptions

#### Notes

The approval is the one the human rung already reads: `inspect/approved.jsonl` on the ledger, written by `aayu inspect --approve` after the device-flow presence check. The lift keeps rule 7: a floor that does not hold, an unmet criterion, a P0 or P1 finding, or an unreviewed file still gives fix. Decision: @rahulnakmol chose this approval route (option C) for tier-2 criteria, in place of Bug #1157.

#### Review guidance

- Check: no approval lifts a verdict with a finding, an unmet criterion, or a floor that does not hold.
- Check: an approval counts only for its exact head and only from the [autonomy] approver.
- Ignore: the app path's human rung; it already reads approvals.

#### Behaviours

1. says: A person's recorded approval of the head accepts the criteria rung B could not see; rule: must; source: criteria 1, 3
2. says: A rung-B finding or an unmet criterion is never lifted by an approval; rule: never; source: criteria 2, 4

### Provenance

Request: "a person's recorded approval of a head never clears a rung-B verdict whose only gap is criteria it marked unclear"
1. stated (request): "a person's recorded approval of a head never clears a rung-B verdict whose only gap is criteria it marked unclear"
2. stated (request): "any rung-B finding still gives fix, with or without an approval"
3. stated (request): "an approval for another head counts for nothing"
4. default (scope): "the floors and an unmet criterion still give fix after an approval (rule 7)"
````
