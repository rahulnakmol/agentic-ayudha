# rahulnakmol/agentic-ayudha#1250: Builders iterate on a draft PR locally, and GitHub runs CI, the merge gate, and the paid review once the PR is ready

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `9fa262e2c5d9e3cc64c30008764530fc3fda1ef7ae3d41f99a75e5af0bc9f7f6` |
| Completion Goal hash | `844484bc3b7941f719b2766e9760e6f5fa774108c4c388faf6d42b2ce56c58c7` |
| Approved to build | #1170 approval 3 by @rahulnakmol |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1250/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a draft PR to dev gets a push; expected: no job of ci.yml runs for it, and no `aayudha/merge-gate` or `aayudha/inspect` job runs, as today
2. [api-trace] state: the draft PR is marked ready for review; expected: ci.yml runs every required job (build, generate-drift, lint, commits, security, docs, release-snapshot) on the head, and the merge gate runs once
3. [api-trace] state: a non-draft PR gets a push; expected: ci.yml and the merge gate run as today
4. [api-trace] state: a push to dev, rolling, or main, or a workflow_dispatch; expected: ci.yml runs as today
5. [api-trace] state: `aayu build <n> --delegate amp` writes a brief; expected: the brief says to open the PR as a draft, run `bash .agents/check --full` and a local review before marking it ready, and mark it ready once
6. [api-trace] state: `aayu ship <pr>` on a draft PR; expected: it refuses with `#<pr> is a draft: mark it ready after the local gate passes`

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want a builder to iterate on a draft PR with the local gate and a local review, and GitHub to run CI, the merge gate, and the paid review once when the PR is marked ready, so that each PR spends less time and money waiting on CI rounds.

### Completion Goal

1. [api-trace] state: a draft PR to dev gets a push; expected: no job of ci.yml runs for it, and no `aayudha/merge-gate` or `aayudha/inspect` job runs, as today
2. [api-trace] state: the draft PR is marked ready for review; expected: ci.yml runs every required job (build, generate-drift, lint, commits, security, docs, release-snapshot) on the head, and the merge gate runs once
3. [api-trace] state: a non-draft PR gets a push; expected: ci.yml and the merge gate run as today
4. [api-trace] state: a push to dev, rolling, or main, or a workflow_dispatch; expected: ci.yml runs as today
5. [api-trace] state: `aayu build <n> --delegate amp` writes a brief; expected: the brief says to open the PR as a draft, run `bash .agents/check --full` and a local review before marking it ready, and mark it ready once
6. [api-trace] state: `aayu ship <pr>` on a draft PR; expected: it refuses with `#<pr> is a draft: mark it ready after the local gate passes`

### Measure

Completion Goal criteria observed

### Threshold

all 6 criteria

### Scenario

Push to a draft PR, mark it ready, push to a ready PR, push to dev, brief a story, and ship a draft.

### Environment

go test of internal/cli and internal/delegate with a fake GitHub, plus the workflow tests in workflows_test.go; never production

### Checkpoints

1. ci.yml skips drafts and runs on ready_for_review (criteria 1, 2, 3, 4)
2. The brief and ship follow the draft-first flow (criteria 5, 6)

### Assumptions

#### Notes

The merge gate already skips drafts and runs on ready_for_review (`.github/workflows/aayudha-merge-gate.yml:3` and `:65`). ci.yml runs on every pull_request push, drafts included, with the default types, so it has no ready_for_review run (`.github/workflows/ci.yml:12`). The required checks stay pinned to GitHub Actions (`build@15368`, read by `requiredChecks`, internal/cli/shipproof.go:143), so a local run never stands in for them. ci.yml names the `live-tests` environment (`.github/workflows/ci.yml:565`), so this PR is in the narrow set and needs a person's ship.

#### Review guidance

- Check: criterion 2: every required check runs on the head that is marked ready.
- Check: criterion 4: pushes to dev, rolling, and main are unchanged.
- Ignore: the wording of the brief.
- Risk: a ready PR whose required checks never run; ship refuses without them, so it cannot merge.

#### Behaviours

1. says: GitHub runs CI and the merge gate once a PR is ready, never on drafts; rule: limit; source: criteria 1, 2
2. says: The required checks still come only from GitHub Actions; rule: never; source: criteria 2, 6

### Provenance

Request: "can we not make the Ci run local and the keep only merge ci run on the github"
1. stated (request): iterate locally, not in GitHub CI
2. stated (request): GitHub runs the checks the merge needs
3. default (scope): ready PRs unchanged
4. default (scope): branch pushes and dispatch unchanged
5. default (scope): the brief asks for the draft-first flow
6. default (scope): ship refuses a draft
````
