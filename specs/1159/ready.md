# rahulnakmol/agentic-ayudha#1159: proof land can never land a merged pull request: its gate runs list no pull request

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `5ecdde557559e64f1799fb974911445fc47e3a2a73859401f2d5528355e21a3e` |
| Completion Goal hash | `c750df5f88095d38550bbb8f570ed3426b866e9bbae9678ce50c27f5afa508f0` |
| Technical Owner | quality-architect |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1159/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a merged pull request whose merge gate run on its head succeeded, with an empty `pull_requests`, the pull request's head branch, and this repository as its head repository; expected: `aayu proof land <pr>` reads the gate checks as green and lands the pack, with one ledger commit
2. [api-trace] state: the merged pull request's gate run lists another pull request, or has an empty `pull_requests` with another head branch or another head repository; expected: proof land refuses with "aayudha/merge-gate is missing" and lands nothing
3. [api-trace] state: an open pull request whose gate run has an empty `pull_requests`, as a fork's has; expected: the gate checks still read as missing, as before

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

`aayu proof land` can never land the pack of a merged pull request. That is its whole job. It reads the head's gate checks through `gateSuites` (internal/cli/home.go). `gateRunOf` keeps a merge gate run only when the run's `pull_requests` lists that pull request alone. Once a pull request merges, GitHub lists no pull request for its runs. So every gate check reads as missing. For #1016, `aayu proof land 1016 --dry-run` on dev 76f8c47 says "aayudha/merge-gate is missing on dafa588; the merge gate must be green". Its gate run 40213240191 is there: pull_request_target, success, check suite 108991528034, head branch feat/founder-minutes-per-pr, head repository rahulnakmol/agentic-ayudha, and `pull_requests: []`. #1024 is the same case. The regression came with #1147 (Bug #1031).

### Steps to reproduce

1. Take a merged pull request whose merge gate run on its head succeeded, such as #1016 at dafa588.
2. Run `aayu proof land 1016 --dry-run --repo rahulnakmol/agentic-ayudha` on dev 76f8c47.

### Expected behaviour

proof land reads the merged pull request's own gate run on its head and lands the pack, as #1147 designed.

### Actual behaviour

proof land refuses with "aayudha/merge-gate is missing on dafa588; the merge gate must be green", because the run lists no pull request.

### Completion Goal

1. [api-trace] state: a merged pull request whose merge gate run on its head succeeded, with an empty `pull_requests`, the pull request's head branch, and this repository as its head repository; expected: `aayu proof land <pr>` reads the gate checks as green and lands the pack, with one ledger commit
2. [api-trace] state: the merged pull request's gate run lists another pull request, or has an empty `pull_requests` with another head branch or another head repository; expected: proof land refuses with "aayudha/merge-gate is missing" and lands nothing
3. [api-trace] state: an open pull request whose gate run has an empty `pull_requests`, as a fork's has; expected: the gate checks still read as missing, as before

### Measure

the exit code and output of `aayu proof land`, and the ledger commits, from the unit test of proof land

### Threshold

all three criteria hold in `go test ./internal/cli/ -run TestProofLand`

### Scenario

A person runs `aayu proof land` for a pull request that merged outside ship.

### Environment

aayu built from dev 76f8c47; rahulnakmol/agentic-ayudha, pull requests #1016 and #1024

### Technical Owner

quality-architect

### Checkpoints

1. A merged pull request's gate run with no listed pull request counts when its head branch and head repository are the pull request's (criteria 1, 2, 3)

### Assumptions

#### Notes

The fix stays in `gateSuites` in internal/cli/home.go, so ship.go, shippass.go, and shipproof.go do not change. It reads the pull request once, and only when a gate run lists no pull request. The packs of #1016 and #1024 expire on 2027-01-23 and 2027-01-24.

#### Review guidance

- Check: a run that lists another pull request never counts.
- Check: an open pull request, or a fork's run, with no listed pull request still fails closed.
- Ignore: the artifact checks of `gatePack`; they already read a merged pull request's runs by branch.

#### Behaviours

1. says: A merged pull request's own gate run counts for proof land; rule: must; source: criteria 1
2. says: A gate run of another pull request, branch, or repository never counts; rule: never; source: criteria 2, 3

### Provenance

Request: "proof land can never land a merged PR, which is its whole job"
1. stated (request): "for a merged PR, accept a gate run with an empty pull_requests when its head_branch is the PR's head ref and its head_sha is the merge's head; keep refusing a run that lists another PR"
2. stated (request): "Add a test with an empty pull_requests on a merged PR"
3. default (scope): "the head repository must be this repository, so a fork's run still fails closed"
````
