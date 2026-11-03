# rahulnakmol/agentic-ayudha#1203: A closed pull request whose commit list lost the head can still let its gate run count for a merged pull request

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `04d210197d918b3e1d5d8f85bac41f916c64dbaf5c1b09c9d981846c815daa24` |
| Completion Goal hash | `a5ff234964e53501c31d1c61fc0660d5dab449a3a29b778b77e942207addb68b` |
| Technical Owner | quality-architect |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1203/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: merged pull request A at head H, and pull request B from its branch, closed while A was open, whose commit list does not show H, with fewer than 250 commits and no force-push, and H is an ancestor of B's head; expected: no gate run on H counts for A, and `aayu proof land <A>` refuses with "aayudha/merge-gate is missing"
2. [api-trace] state: the same B, but H is not an ancestor of B's head; expected: `aayu proof land <A>` lands the pack from A's own gate run, as before
3. [api-trace] state: the read of the comparison of H with B's head fails; expected: `aayu proof land <A>` fails with an error and lands nothing

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

`hadHead` (internal/cli/home.go, from PR #1183 for Bug #1161) can say that another pull request never had a head when it did. Take merged pull request A at head H, and pull request B from the same branch, closed while A was open. `hadHead` reads B's commits with `PullRequests.ListCommits`. GitHub lists only the commits that B's base does not already have. So when H later reaches B's base, H drops off the list. With fewer than 250 commits and no `head_ref_force_pushed` event, `hadHead` returns false. A gate run on H that was B's then counts for A in `gateSuites`, and proof land can take its pack. The merge gate's inspect raised this on PR #1183 at 30c9105 as finding 9d13bc7c7f (P1). It needs merge-commit merges, because a squash merge never puts H on a base. This repository squash-merges, so the case comes from a repository that installs aayudha.

### Steps to reproduce

1. In a repository that merges with merge commits, open pull request A and pull request B from one branch at head H, with different bases.
2. Let a gate run on H finish while both are open. Push a new commit to B, then close B.
3. Merge A, then merge A's base into B's base, so that H reaches B's base.
4. Run `aayu proof land <A>`.

### Expected behaviour

B may have had H, so no gate run on H counts for A. proof land refuses with "aayudha/merge-gate is missing".

### Actual behaviour

B's commit list does not show H, so `hadHead` returns false, and the run on H counts for A.

### Completion Goal

1. [api-trace] state: merged pull request A at head H, and pull request B from its branch, closed while A was open, whose commit list does not show H, with fewer than 250 commits and no force-push, and H is an ancestor of B's head; expected: no gate run on H counts for A, and `aayu proof land <A>` refuses with "aayudha/merge-gate is missing"
2. [api-trace] state: the same B, but H is not an ancestor of B's head; expected: `aayu proof land <A>` lands the pack from A's own gate run, as before
3. [api-trace] state: the read of the comparison of H with B's head fails; expected: `aayu proof land <A>` fails with an error and lands nothing

### Measure

the exit code and output of `aayu proof land`, from the unit test of proof land

### Threshold

the three criteria hold in `go test ./internal/cli/ -run TestProofLand`

### Scenario

A person runs `aayu proof land` in a repository that merges with merge commits, for a pull request whose branch another pull request also used.

### Environment

aayu built from PR #1183 at 30c9105; rahulnakmol/agentic-ayudha

### Technical Owner

quality-architect

### Checkpoints

1. A closed pull request from the branch whose history holds the head counts as one that may have had it (criteria 1, 2, 3)

### Assumptions

#### Notes

This Bug builds on Bug #1161 and its PR #1183, held for v0.4.1. Proposed fix: in `hadHead`, when B's commit list does not show H, read `GET /repos/{owner}/{repo}/compare/{H}...{B head}`. A status of `ahead` or `identical` means H is an ancestor of B's head, so B may have had it. Then the run is refused. Any other status means B never had H. A failed read is an error, so proof land fails closed. Proposed test: a TestProofLand case where B closed in A's window, its commits omit H, it has no force-push, and the fake's comparison says `ahead`. It expects "aayudha/merge-gate is missing". A second case where the comparison says `diverged` lands. GitHub's commit list for a closed, unmerged pull request is not verified.

#### Review guidance

- Check: a closed pull request whose history holds the head never lets the run count.
- Ignore: the window and open-at-merge rules of #1161; they stay.

#### Behaviours

1. says: A gate run counts only for the pull request whose gate it was; rule: must; source: criteria 1, 2, 3

### Provenance

Request: "file 9d13bc7c7f as a Bug with your compare-based fix and test in its notes, linked to #1161"
1. stated (request): "file 9d13bc7c7f as a Bug with your compare-based fix and test in its notes, linked to #1161"
2. default (scope): "a closed pull request that never had the head still lets the merged one's run count"
3. default (scope): "fail closed when the comparison cannot be read"
````
