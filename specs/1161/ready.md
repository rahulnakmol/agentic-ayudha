# rahulnakmol/agentic-ayudha#1161: A gate run of a pull request that once shared the branch and head can count for a merged pull request

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `602d401a98268ff369496fb1035a8c4af60b5b8dfd11f35a33232ae588353fbc` |
| Completion Goal hash | `eabc9acdfe7ad4bf421d9b1b6883cec49825b1f467dd16863916343d00ba9f56` |
| Technical Owner | quality-architect |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1161/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: merged pull request A and another pull request B, from the same branch, both had head H while A was open, and B's head is now another commit; expected: no gate run on H counts for A in `gateSuites`, and `aayu proof land <A>` refuses with "aayudha/merge-gate is missing"
2. [api-trace] state: merged pull request A, and no other pull request from its branch ever had A's head; expected: `aayu proof land <A>` lands the pack from A's own gate run, as before

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

A merge gate run of another pull request can count as a merged pull request's own run. After a merge, GitHub lists no pull request for its gate runs. So `mergedRunOf` (internal/cli/home.go, from PR #1160 for Bug #1159) decides ownership from the run's head commit, branch, repository, and time. It also refuses the run when another pull request from the same branch has the same head now. But a second pull request can share the branch and the head for a time, then move on to a new head. Its old gate run on the shared head then passes every rule for the merged pull request, if it started while that pull request was open. Its green checks can then mask the merged pull request's own red check, and proof land can take its pack. CodeRabbit raised this on PR #1160 at 4b2b97e as a retained Medium concern.

### Steps to reproduce

1. Open pull request A and pull request B from the same branch, at the same head commit H.
2. Let the merge gate run for both on H. A's run fails, and B's run succeeds.
3. Push a new commit to B, so B's head is no longer H. Then merge A.
4. Run `aayu proof land <A>`.

### Expected behaviour

B's run on H does not count for A. proof land reads only A's own runs, and it refuses because A's gate is red.

### Actual behaviour

B's old run on H passes the rules of `mergedRunOf` for A, so its green checks and its pack can count for A.

### Completion Goal

1. [api-trace] state: merged pull request A and another pull request B, from the same branch, both had head H while A was open, and B's head is now another commit; expected: no gate run on H counts for A in `gateSuites`, and `aayu proof land <A>` refuses with "aayudha/merge-gate is missing"
2. [api-trace] state: merged pull request A, and no other pull request from its branch ever had A's head; expected: `aayu proof land <A>` lands the pack from A's own gate run, as before

### Measure

the exit code and output of `aayu proof land`, from the unit test of proof land

### Threshold

both criteria hold in `go test ./internal/cli/ -run TestProofLand`

### Scenario

A person runs `aayu proof land` for a pull request that merged outside ship, from a branch another pull request also used.

### Environment

aayu built from dev after PR #1160; rahulnakmol/agentic-ayudha

### Technical Owner

quality-architect

### Checkpoints

1. A merged pull request's run with no listed pull request does not count when another pull request ever had that branch at that head (criteria 1, 2)

### Assumptions

#### Notes

The fix can read each other pull request's history from the branch, for example its commits or its head-change timeline events, to see whether it ever had the head. If no source can show that, the run does not count, and the check fails closed. The narrow case needs two pull requests open from one branch at once.

#### Review guidance

- Check: a run of another pull request that once had the head never counts, whatever its head is now.
- Ignore: the rules that #1160 added for the open window and the repository; they stay.

#### Behaviours

1. says: A gate run counts only for the pull request whose gate it was; rule: must; source: criteria 1, 2

### Provenance

Request: "a PR that once shared the branch and head and later advanced"
1. stated (request): "a PR that once shared the branch and head and later advanced"
2. default (scope): "fail closed when no source shows another pull request's past heads"
````
