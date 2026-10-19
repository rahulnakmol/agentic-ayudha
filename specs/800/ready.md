# rahulnakmol/agentic-ayudha#800: A person's /aayu review again on a PR stopped at the review cap starts no merge gate run, so the card stays

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `5a9a2ab958e00c8120d18339b018569a1c4c1fb4bc93f719845114fbe544acc0` |
| Completion Goal hash | `ef311ef4ae17aa97229bab6a822070f35938542c867761695314613568afcdae` |
| Technical Owner | devops-devex-tinkerer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/800/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a PR stopped at the review cap, then a comment `/aayu review again` from an account with write access and no push after it; expected: a merge gate run starts on the PR's head and its inspect job runs again
2. [api-trace] state: the same comment from an account without write access; expected: no run starts, and nothing is re-run
3. [api-trace] state: after the run of criterion 1 records a fresh review, a read of `aayu inbox --list`; expected: the PR is no longer listed under "At the review cap"

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

A person's `/aayu review again` (`inspect.ReopenCommand`) on a PR whose review stopped at the round cap re-runs nothing, so the PR stays on the inbox's "At the review cap" card. The merge gate's reply job (`aayudha/inspect-reply` in .github/workflows/aayudha-merge-gate.yml) runs on an `issue_comment` only when the comment is a bot's or carries the `aayudha-review` fence; a person's plain `/aayu review again` is neither. On 2026-10-19 #722's card stayed after the comment was posted, and a push was the only way to run the gate again.

### Steps to reproduce

1. Let a PR's inspect review stop at the round cap, so `aayu inbox` lists it as "At the review cap".
2. As a person with write access, comment `/aayu review again` on the PR, and push nothing.
3. Watch the PR's Actions runs, then refresh `aayu inbox`.

### Expected behaviour

The comment starts the merge gate's inspect on the PR's head in a run of the merge gate workflow, and once that run records a fresh review the PR leaves the "At the review cap" card on the next inbox read.

### Actual behaviour

No workflow run starts for the comment: the reply job's condition skips it, and the card stays.

### Completion Goal

1. [api-trace] state: a PR stopped at the review cap, then a comment `/aayu review again` from an account with write access and no push after it; expected: a merge gate run starts on the PR's head and its inspect job runs again
2. [api-trace] state: the same comment from an account without write access; expected: no run starts, and nothing is re-run
3. [api-trace] state: after the run of criterion 1 records a fresh review, a read of `aayu inbox --list`; expected: the PR is no longer listed under "At the review cap"

### Measure

Reopen comments from people with write access that start no merge gate run.

### Threshold

0.

### Scenario

@rahulnakmol comments `/aayu review again` on a capped PR from his phone, and within minutes the gate reviews it again without anyone pushing.

### Environment

rahulnakmol/agentic-ayudha's merge gate on a PR into dev; the workflow is in `[autonomy] always_person`, so @rahulnakmol merges the fix.

### Technical Owner

devops-devex-tinkerer

### Links

### Checkpoints

1. `go test ./... -run 'TestMergeGate|TestWorkflows'` passes with a person's reopen comment that re-runs inspect, and one from an account without write access that does not (criteria 1, 2, 3)

### Assumptions

Re-running the gate on a person's comment uses the workflow's existing permission check for who may reopen; it adds no new secret or permission to the workflow.

### Open questions

### Provenance

Request: "this kind of work should also come in inbox"
Vague: "kind of" → criteria 1, 2, 3
1. stated (request): "this kind of work should also come in inbox"
2. stated (request): "this kind of work should also come in inbox"
3. stated (request): "this kind of work should also come in inbox"
````
