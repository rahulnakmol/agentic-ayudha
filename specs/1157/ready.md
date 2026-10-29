# rahulnakmol/agentic-ayudha#1157: A Story whose criteria are tier-2 eval results can never reach merge before it merges

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `8cf2fec50b41cd17dcb8d04fdd15c0093c93dd9c7017ca0b731516fc1c3ccd61` |
| Completion Goal hash | `2395310a6f707a568f3e556491dabcc136c30170ad0cefff15040699175cdb23` |
| Technical Owner | quality-architect |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1157/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the evidence map gives a criterion `evals = ["mission/stays-in-list"]`, and an evals.yml run on the head commit left results in which that case passed all k runs with no error and a mean at the bar of evals/eval.toml; expected: `aayu verify <pr>` writes that criterion's evidence with the case, the commit, the runs, pass^k, the mean, and the run, and the criterion is not without evidence
2. [api-trace] state: the results are for another commit, are missing, are still running, or the case failed a run, errored, or has a mean under the bar; expected: the criterion gets no evidence, and verify names the reason
3. [api-trace] state: the pull request changes a file the eval harness builds or reads (a Go file that is not a test, go.mod, go.sum, a file under evals/, or .github/workflows/evals.yml); expected: the results do not count for any criterion, and verify says why

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

A Story or Bug whose Completion Goal criteria are tier-2 eval results cannot reach `merge` before it merges. The merge gate's pack holds only the tests that `.aayudha/evidence/<n>.toml` names (`verify.TestEvidence`, internal/verify/evidence.go). Nothing in `aayu verify`, the pack, or inspect reads the results of an evals.yml run. So inspect marks those criteria `unclear` and says `fix`. A recorded `aayu inspect --approve` does not change that, because the paid reviewer answered (inspect_app.go reads an approval only when no reviewer can answer). On PR #1146 (Bug #1132) at 734b107, rung B said `fix` with no findings: criteria 1 and 2 were unclear for the missing results of the `mission` group. The approval of @rahulnakmol at 734b107 did not reach merge. A PR on proof cannot merge, and ship refuses because the inspect check is red.

### Steps to reproduce

1. Make a Story or Bug whose criteria are tier-2 results, such as Bug #1132: "`mission/stays-in-list` passes all 5 runs".
2. Open its PR, then dispatch evals.yml with group `mission` on the PR head, so results exist for that commit.
3. Read the merge gate's pack and the inspect comment for that head.

### Expected behaviour

The evidence map can name eval cases for a criterion. The merge gate's verify reads the evals.yml results for the exact head commit and puts each case's numbers in that criterion's evidence. Inspect can then judge the criterion from them.

### Actual behaviour

The pack has no eval results, the criteria are `unclear`, and inspect says `fix` at every head. The PR can merge only outside ship.

### Completion Goal

1. [api-trace] state: the evidence map gives a criterion `evals = ["mission/stays-in-list"]`, and an evals.yml run on the head commit left results in which that case passed all k runs with no error and a mean at the bar of evals/eval.toml; expected: `aayu verify <pr>` writes that criterion's evidence with the case, the commit, the runs, pass^k, the mean, and the run, and the criterion is not without evidence
2. [api-trace] state: the results are for another commit, are missing, are still running, or the case failed a run, errored, or has a mean under the bar; expected: the criterion gets no evidence, and verify names the reason
3. [api-trace] state: the pull request changes a file the eval harness builds or reads (a Go file that is not a test, go.mod, go.sum, a file under evals/, or .github/workflows/evals.yml); expected: the results do not count for any criterion, and verify says why

### Measure

the criteria that `aayu verify --json` reports without evidence, and the evidence files in the pack, from the unit tests of the evidence map

### Threshold

all three criteria hold in `go test ./internal/verify/... ./internal/cli/...`

### Scenario

The merge gate's verify on a pull request whose story's criteria are tier-2 eval results.

### Environment

aayudha-merge-gate.yml gate job on a pull request to dev; dev at 7759eb0

### Technical Owner

quality-architect

### Checkpoints

1. The evidence map names eval cases, and verify turns the head's results into the criterion's evidence (criteria 1)
2. Results for another commit, missing or failing results, and a change to the harness give no evidence (criteria 2, 3)

### Assumptions

#### Notes

Seen on PR #1146 at 734b107, inspect comment 6467087595. The gate job needs `actions: read` to list the runs and download the results artifact, so the fix changes aayudha-merge-gate.yml, which needs a person. The results come from an evals.yml run on the head, which uses the PR's own copy of the workflow and the harness. So they count only when the PR does not change them.

#### Review guidance

- Check: verify reads results only for the head commit, never for another commit of the branch.
- Check: a change to the harness or to evals.yml makes the results not count.
- Ignore: how evals.yml scores a run; this reads its results file as it is.

#### Behaviours

1. says: An eval result counts as evidence only for the commit it ran on; rule: must; source: criteria 1, 2
2. says: Results from a run whose harness the PR changed never count; rule: never; source: criteria 3

### Provenance

Request: "a Story whose criteria are tier-2 eval results can never reach merge before it merges"
1. stated (request): "a Story whose criteria are tier-2 eval results can never reach merge before it merges"
2. stated (request): "an evals.yml dispatch on the PR's head writes results that the pack, or inspect's criteria check, reads for that exact head"
3. default (scope): "read the results only for the head SHA"
````
