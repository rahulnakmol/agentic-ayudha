# rahulnakmol/agentic-ayudha#618: Sampled audit after merge: a merge on proof reaches the inbox, and a correction resets the streak and opens a revert PR

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `436164fed49775da900cba847c65e90eb76f11d56f8443034db46006df8db2ce` |
| Completion Goal hash | `bda7b2a20289a3257643af5ffd58d95a3494e204fd369b8e5d9fe90067c448e6` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/618/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu ship <pr>` merges a Story PR on proof (human pass kind `proof`); expected: `autonomy/audit/<pr>.json` (PR, story, head, merge commit, footprint, how it passed) is written to the ledger and the PR gets the `aayu:audit` label, and a PR a person's review passed writes none
2. [api-trace] state: a person runs `aayu inbox` with one audit entry on the ledger; expected: a "merged PR to audit" item whose recommended action is "looks right" and whose other action is "correction"
3. [api-trace] state: the person picks "looks right"; expected: `{"pr":N,"correction":false,"by":"<login>"}` is appended to `autonomy/samples.jsonl`, the audit entry is removed, and the sample count in `autonomy/samples.jsonl` grows by one
4. [api-trace] state: the person picks "correction"; expected: the sample is recorded with `"correction":true`, the merged PR gets the `correction` label so `aayu status --streak` shows each footprint layer back at 0, and a PR into dev that reverts the merge commit is opened with `Refs #<story>`
5. [api-trace] state: an agent lane runs `aayu inbox` with an audit entry and no device approval; expected: nothing is recorded, labelled, or opened

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want a PR that `aayu ship` merged on proof to reach me as an audit sample after merge, so that a correction resets the layer's streak and opens a revert PR, without any look blocking the merge.

Decision #593 option 1: the streak, the never layers and the plan review no longer block a dev merge; they become #572's sampled audit after merge, which never blocks. Replaces #589, which was written for the superseded intent pass.

### Completion Goal

1. [api-trace] state: `aayu ship <pr>` merges a Story PR on proof (human pass kind `proof`); expected: `autonomy/audit/<pr>.json` (PR, story, head, merge commit, footprint, how it passed) is written to the ledger and the PR gets the `aayu:audit` label, and a PR a person's review passed writes none
2. [api-trace] state: a person runs `aayu inbox` with one audit entry on the ledger; expected: a "merged PR to audit" item whose recommended action is "looks right" and whose other action is "correction"
3. [api-trace] state: the person picks "looks right"; expected: `{"pr":N,"correction":false,"by":"<login>"}` is appended to `autonomy/samples.jsonl`, the audit entry is removed, and the sample count in `autonomy/samples.jsonl` grows by one
4. [api-trace] state: the person picks "correction"; expected: the sample is recorded with `"correction":true`, the merged PR gets the `correction` label so `aayu status --streak` shows each footprint layer back at 0, and a PR into dev that reverts the merge commit is opened with `Refs #<story>`
5. [api-trace] state: an agent lane runs `aayu inbox` with an audit entry and no device approval; expected: nothing is recorded, labelled, or opened

### Measure

criteria observed

### Threshold

5 of 5

### Scenario

Run the cli tests with a fake GitHub and a local ledger: a Story shipped on proof, then its audit in the inbox.

### Environment

go test ./internal/cli with internal/github/fake

### Checkpoints

1. ship writes the audit entry for a merge on proof (criterion 1)
2. the inbox lists audit entries and records "looks right" as a clean sample (criteria 2, 3, 5)
3. a correction labels the PR and opens the revert PR (criterion 4)

### Assumptions

- every merge on proof is sampled; sampling rates by tier come later
- the revert PR is opened with `git revert` of the squash commit in the inbox's checkout, on a branch cut from the base branch
- a correction's revert PR is an ordinary PR and merges on green; it is not a Story PR

### Provenance

Request: "Then #589."
Term: "sample" → an `autonomy/audit/<pr>.json` entry on the ledger, recorded in `autonomy/samples.jsonl` once a person looks at it; "A sample of autonomous merges goes into the inbox after merge"
Vague: "goes into the inbox" → criterion 2: a "merged PR to audit" item in `aayu inbox`
1. stated (Decision #572 option 1): "A sample of autonomous merges goes into the inbox after merge."
2. stated (Decision #572 option 1): "A correction resets the layer's streak and opens a revert PR."
3. stated (Decision #572 option 1): "Those looks never block a merge."
4. stated (Decision #593): "The streak, the never layers and the plan review stop blocking a `dev` merge. They become #572's sampled audit after merge, which never blocks: a correction resets the streak and opens a revert PR."
5. stated (founder, 2026-10-18): "Keep floors, the different-family rule and human-only actions (Decisions, publishing, catalog) unchanged."
````
