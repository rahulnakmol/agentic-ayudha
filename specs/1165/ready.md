# rahulnakmol/agentic-ayudha#1165: A cached merge from an approval lift is not checked again against the current approver

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `ea87de99b299a5e9835401fa69f78335c20e837add36a1e7399fc862a4144a29` |
| Completion Goal hash | `dfa31bec36caa4376bb203ebec550343e0c7fbdf99e46e709f028272a05b21cf` |
| Technical Owner | engineering-manager |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1165/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a stored merge from an approval lift at head H, and the [autonomy] approver is now another person with no approval of H; expected: inspect at H says fix, with no new review, and its comment has no lift note
2. [api-trace] state: a stored merge from an approval lift at head H, and the same approver; expected: inspect at H says merge from the stored review, with no new review

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

A cached merge that came from an approval lift is not judged again. PR #1164 (Bug #1163) lets the [autonomy] approver's `aayu inspect --approve` of the head lift a rung-B fix whose only gap is unclear criteria. inspect then stores the merge in its state. The review cache keys on the PR's content, not on the approver or the ledger. So a later run at the same head reads the stored merge and does not check the approval again. When the [autonomy] approver changes in `.aayudha/config.toml`, the old approver's lift still stands. CodeRabbit raised this on PR #1164 at 12ba4d8 as a Major finding (internal/cli/inspect.go:255).

### Steps to reproduce

1. Let rung B give fix at head H, with no finding and a criterion unclear.
2. As the [autonomy] approver A, run `aayu inspect <pr> --approve` for H. inspect says merge.
3. Change [autonomy] approver to B on the base branch.
4. Run the merge gate's inspect check again at H.

### Expected behaviour

inspect says fix: no approval by the current approver holds for H.

### Actual behaviour

inspect reads the stored merge and says merge.

### Completion Goal

1. [api-trace] state: a stored merge from an approval lift at head H, and the [autonomy] approver is now another person with no approval of H; expected: inspect at H says fix, with no new review, and its comment has no lift note
2. [api-trace] state: a stored merge from an approval lift at head H, and the same approver; expected: inspect at H says merge from the stored review, with no new review

### Measure

the disposition and the comment of `aayu inspect`, from the unit tests of inspect

### Threshold

both criteria hold in `go test ./internal/cli/ -run TestInspectPaidReviewer`

### Scenario

The [autonomy] approver changes while a PR's lifted head waits to ship.

### Environment

aayu built from dev 2824358; rahulnakmol/agentic-ayudha

### Technical Owner

engineering-manager

### Checkpoints

1. Every run checks a stored lift again against the current approver and the ledger (criteria 1, 2)

### Assumptions

#### Notes

Approvals on the ledger are append-only, so a change of approver is the way the lift stops holding. The fix is built: it restores the review's fix on each run and lifts it again only while the current approver's approval holds.

#### Review guidance

- Check: a stored lift never stands without the current approver's approval of the head.
- Ignore: the lift rules of Bug #1163; they stay.

#### Behaviours

1. says: A lift stands only while the current approver's approval of the head does; rule: must; source: criteria 1, 2

### Provenance

Request: "revalidate the approval before reusing a lifted result"
1. stated (request): "revalidate the approval before reusing a lifted result"
2. default (scope): "no new paid review on a cached run"
````
