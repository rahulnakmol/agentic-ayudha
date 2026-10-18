# rahulnakmol/agentic-ayudha#589: Sampled audit after merge: an intent-pass merge reaches the inbox, and a correction resets the streak and opens a revert PR

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `58ff9580aacd767b67cd90e5eff67c71501bbc9ad9aef6238345a16f7c8e01d4` |
| Completion Goal hash | `aa0b8e35e60b1392482bdfb463f64bdbb1998c104b6a55af8916ab8cd7504e76` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/589/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu ship <pr>` merges a Story PR whose intent pass held, whether a person passed it or the relief did; expected: `autonomy/audit/<pr>.json` (PR, story, head, merge commit, footprint, how it passed) is written to the ledger, and a PR merged without an intent pass writes none
2. [api-trace] state: a person runs `aayu inbox` with one audit entry on the ledger; expected: a "merged PR to audit" item whose recommended action is "looks right" and whose other action is "correction"
3. [api-trace] state: the person picks "looks right"; expected: `{"pr":N,"correction":false,"by":"<login>"}` is appended to `autonomy/samples.jsonl`, the audit entry is removed, and ship's next intent pass counts one more sample
4. [api-trace] state: the person picks "correction"; expected: the sample is recorded with `"correction":true`, the merged PR gets the `correction` label so `aayu status --streak` shows each footprint layer back at 0, and a PR into dev that reverts the merge commit is opened with `Refs #<story>`
5. [api-trace] state: an agent lane runs `aayu inbox` with an audit entry and no device approval; expected: nothing is recorded, labelled, or opened

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want a merged PR that the intent pass held for to reach me as an audit sample after merge, so that a correction resets the layer's streak and opens a revert PR, and the 50-sample count that turns relief on is real.

Decision #572 option 1, story 3: "A sample of autonomous merges goes into the inbox after merge. A correction resets the layer's streak and opens a revert PR." Those looks never block a merge.

### Completion Goal

1. [api-trace] state: `aayu ship <pr>` merges a Story PR whose intent pass held, whether a person passed it or the relief did; expected: `autonomy/audit/<pr>.json` (PR, story, head, merge commit, footprint, how it passed) is written to the ledger, and a PR merged without an intent pass writes none
2. [api-trace] state: a person runs `aayu inbox` with one audit entry on the ledger; expected: a "merged PR to audit" item whose recommended action is "looks right" and whose other action is "correction"
3. [api-trace] state: the person picks "looks right"; expected: `{"pr":N,"correction":false,"by":"<login>"}` is appended to `autonomy/samples.jsonl`, the audit entry is removed, and ship's next intent pass counts one more sample
4. [api-trace] state: the person picks "correction"; expected: the sample is recorded with `"correction":true`, the merged PR gets the `correction` label so `aayu status --streak` shows each footprint layer back at 0, and a PR into dev that reverts the merge commit is opened with `Refs #<story>`
5. [api-trace] state: an agent lane runs `aayu inbox` with an audit entry and no device approval; expected: nothing is recorded, labelled, or opened

### Measure

criteria observed

### Threshold

5 of 5

### Scenario

Run the cli tests with a fake GitHub and a local ledger: a Story shipped on the intent pass, then its audit in the inbox.

### Environment

go test ./internal/cli with internal/github/fake

### Checkpoints

1. ship writes the audit entry for an intent-pass merge (criterion 1)
2. the inbox lists audit entries and records "looks right" as a clean sample (criteria 2, 3, 5)
3. a correction labels the PR and opens the revert PR (criterion 4)

### Assumptions

- every intent-pass merge is sampled until story 4 adds sampling rates by tier
- the revert PR is opened with `git revert` of the squash commit in the inbox's checkout, on a branch cut from the base branch
- a correction's revert PR is an ordinary PR and merges on green; it is not a Story PR

### Provenance

Request: "Build stories 2 to 4 in order"
Term: "sample" → an `autonomy/audit/<pr>.json` entry on the ledger, recorded in `autonomy/samples.jsonl` once a person looks at it; "A sample of autonomous merges goes into the inbox after merge"
Vague: "goes into the inbox" → criterion 2: a "merged PR to audit" item in `aayu inbox`
1. stated (Decision #572 option 1): "A sample of autonomous merges goes into the inbox after merge."
2. stated (Decision #572 option 1): "A correction resets the layer's streak and opens a revert PR."
3. stated (Decision #572 option 1): "Those looks never block a merge."
4. stated (founder, 2026-10-18): "The per-PR relief of option 1 switches on only after 50 sampled autonomous merges show corrections under 5 %"
5. stated (founder, 2026-10-18): "Keep floors, the different-family rule and human-only actions (Decisions, publishing, catalog) unchanged."
````
