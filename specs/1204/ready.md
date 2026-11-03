# rahulnakmol/agentic-ayudha#1204: A pull request from the branch outside the open window blocks a merged pull request's own gate run

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `a114eee45f4b4b4c8f4528a9a0961972f37bc080980b67d71a504c1bfaf01147` |
| Completion Goal hash | `e63c7cb3d4857bb504180e4bf07ad0c9dd7d186dfae42ab5e660c81b877df7cc` |
| Technical Owner | quality-architect |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1204/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: merged pull request A at head H, and pull request B from its branch at head H that closed before A opened; expected: `aayu proof land <A>` lands the pack from A's own gate run
2. [api-trace] state: the same, but B opened after A merged; expected: `aayu proof land <A>` lands the pack from A's own gate run
3. [api-trace] state: B at head H was open while A was open; expected: `aayu proof land <A>` refuses with "aayudha/merge-gate is missing", as before

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

`mergedRunOf` (internal/cli/home.go) refuses a merged pull request's gate run when any other pull request from its branch has the same head now. It makes this check before the open-window skips. So a pull request from the branch that closed before the merged one opened, or opened after it merged, still blocks the run when it sits at that head. Such a pull request cannot own a run in the merged pull request's open window. proof land then reports "aayudha/merge-gate is missing" for a run that is the merged pull request's own. This fails closed: no other pull request's run ever counts. The check is from PR #1160 (Bug #1159). CodeRabbit raised it on PR #1183 at 30c9105 as a Minor finding (home.go:290-305).

### Steps to reproduce

1. Open pull request B from a branch at head H, then close it.
2. Open pull request A from the same branch at H, let its gate run, and merge it.
3. Run `aayu proof land <A>`.

### Expected behaviour

B was never open while A was, so A's own run on H counts, and proof land lands its pack.

### Actual behaviour

B's head is H, so `mergedRunOf` refuses every run on H, and proof land says "aayudha/merge-gate is missing".

### Completion Goal

1. [api-trace] state: merged pull request A at head H, and pull request B from its branch at head H that closed before A opened; expected: `aayu proof land <A>` lands the pack from A's own gate run
2. [api-trace] state: the same, but B opened after A merged; expected: `aayu proof land <A>` lands the pack from A's own gate run
3. [api-trace] state: B at head H was open while A was open; expected: `aayu proof land <A>` refuses with "aayudha/merge-gate is missing", as before

### Measure

the exit code and output of `aayu proof land`, from the unit test of proof land

### Threshold

the three criteria hold in `go test ./internal/cli/ -run TestProofLand`

### Scenario

A person runs `aayu proof land` for a pull request from a branch that another pull request used before or after it.

### Environment

aayu built from PR #1183 at 30c9105; rahulnakmol/agentic-ayudha

### Technical Owner

quality-architect

### Checkpoints

1. The current-head check applies only to a pull request open in the merged one's window (criteria 1, 2, 3)

### Assumptions

#### Notes

This Bug builds on Bug #1161 and its PR #1183, held for v0.4.1. Proposed fix: in `mergedRunOf`, move the check `o.GetHead().GetSHA() == head` after the window skips, so a pull request outside the window is skipped first. Proposed test: two TestProofLand cases, a twin at the head that closed before A opened and one that opened after A merged; both land.

#### Review guidance

- Check: a pull request open in the window at the head still refuses the run.
- Ignore: `hadHead`; Bug #1161 and its follow-up own it.

#### Behaviours

1. says: A pull request outside the merged one's open window does not block its gate run; rule: must; source: criteria 1, 2, 3

### Provenance

Request: "file CodeRabbit's Minor the same way, P3"
1. stated (request): "file CodeRabbit's Minor the same way, P3"
2. default (scope): "a pull request opened after the merge does not block the run either"
3. default (scope): "a pull request open in the window at the head still refuses the run"
Term: "P3" → the priority:P3 label, the lowest priority; "file CodeRabbit's Minor the same way, P3"
````
