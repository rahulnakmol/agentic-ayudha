# rahulnakmol/agentic-ayudha#659: aayu build reviews its change in the build session with a reviewer of another family before each push

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `2ef02b01eb026fb024da87326cb43e0ecf7efe40a1d891ae8b27ad64be181653` |
| Completion Goal hash | `bd26fbd90c19830b42d6f49adff0b4e3ac967585a34e6a1f61aff5bd8b425231` |
| Technical Owner | code-architect |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/659/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a held checkpoint of a story built with `AAYU_MODEL` of family anthropic, the three AAYU_REVIEW_MODEL settings naming grok-4.7, and a reviewer reply of disposition fix with one P1 finding; expected: `aayu build` exits 4, lists the finding's ID, severity, file, line, and fix, calls the reviewer once at the configured endpoint with the configured key, and does not push the branch
2. [api-trace] state: the open P1 of criterion 1 declined with `aayu build <story> --decline "<id>: <why>"`; expected: build calls no reviewer again, pushes the branch, and the PR body carries a "Session review (self-attested)" section saying it is not a review that counts, with the round, the reviewer and its family, and the declined item with its reason; a decline of an ID that is not open exits 2
3. [api-trace] state: three more rounds after that push, each with a new P1; expected: the first two hold the push, the third pushes, a review fix commit is the same checkpoint's next attempt, and the PR body's one section lists the earlier items as fixed and the last one as left open at the round cap
4. [api-trace] state: a builder and the only callable reviewer of the same family (grok-4.7); expected: no reviewer is called, the branch is pushed, and the PR body says the push went out without a session review and why
5. [screenshot] state: the build guide (`docs/guide/guide-build.md`); expected: a "session review loop" section explains the loop, fixing or declining, the cap, the self-attested summary, and the three settings shared with CI

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

`aayu build` reviews its own change in the build session before each push, with a reviewer model of a family other than the builder's, and loops until nothing is left open or a round cap is reached; then it pushes. It is the session half of Decision #656, option 2: the merge gate's CI review (Story #655, PR #657) re-reviews once and stays the gate. The session review reuses inspect's own review (`inspect.Runner.Run` in `internal/inspect/inspect.go`: the same prompt, schema, noise rules, carried findings, and rule 7), and the reviewer is found from the same `AAYU_REVIEW_MODEL_URL`, `AAYU_REVIEW_MODEL`, and `AAYU_REVIEW_MODEL_KEY` names the CI review reads, else `[inspect] paid_reviewer`, else the review lane. Model families are what `verbs.FamilyOf` knows today; Story #648's cheap reviewers are out of scope.

### Completion Goal

1. [api-trace] state: a held checkpoint of a story built with `AAYU_MODEL` of family anthropic, the three AAYU_REVIEW_MODEL settings naming grok-4.7, and a reviewer reply of disposition fix with one P1 finding; expected: `aayu build` exits 4, lists the finding's ID, severity, file, line, and fix, calls the reviewer once at the configured endpoint with the configured key, and does not push the branch
2. [api-trace] state: the open P1 of criterion 1 declined with `aayu build <story> --decline "<id>: <why>"`; expected: build calls no reviewer again, pushes the branch, and the PR body carries a "Session review (self-attested)" section saying it is not a review that counts, with the round, the reviewer and its family, and the declined item with its reason; a decline of an ID that is not open exits 2
3. [api-trace] state: three more rounds after that push, each with a new P1; expected: the first two hold the push, the third pushes, a review fix commit is the same checkpoint's next attempt, and the PR body's one section lists the earlier items as fixed and the last one as left open at the round cap
4. [api-trace] state: a builder and the only callable reviewer of the same family (grok-4.7); expected: no reviewer is called, the branch is pushed, and the PR body says the push went out without a session review and why
5. [screenshot] state: the build guide (`docs/guide/guide-build.md`); expected: a "session review loop" section explains the loop, fixing or declining, the cap, the self-attested summary, and the three settings shared with CI

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

`internal/cli` tests with the fake GitHub and the fake model provider, through `aayu build` on a story worktree; the build guide rendered on the docs site.

### Environment

local checkout, this repository's CI build job on a pull request, and the docs site build

### Technical Owner

code-architect

### Checkpoints

1. the session review holds the push on fix, and a decline or the round cap releases it, with the PR body summary (criteria 1, 2, 3, 4)
2. the build skill and the build guide explain the loop (criterion 5)

### Assumptions

- the session review never counts as a review: in solo mode it is self-attested (Decision #656, Trust), so the merge gate's inspect decides
- the loop's state lives in the story worktree's own git directory, not on the ledger; a build resumed on another machine starts its rounds afresh, and the PR body keeps the record

### Open questions

- deferred: should a declined finding's reason reach the CI reviewer as context? (it can wait: the PR body already carries it as untrusted data)

### Provenance

Request: "Has to be part of 0.3 please"
1. stated (v0.3.0 release coordinator, 2026-10-18, scoping the founder's request): "aayu build ... runs a review step on the local diff before each push. It calls a reviewer model of a family other than the builder's, through the user's configured models or gateway"; the founder's naming: the session loop uses the same AAYU_REVIEW_MODEL_URL, AAYU_REVIEW_MODEL and AAYU_REVIEW_MODEL_KEY names as CI
2. stated (coordinator): "fixes the findings or records why it declined them" and "The PR body carries a summary of the session review rounds ... It is never presented as a review that counts"
3. stated (coordinator): "loops up to a cap. Then it pushes", with the summary listing "rounds, findings, and fixed or declined"
4. stated (coordinator): "a reviewer model of a family other than the builder's"; accepted default (coordinator, 2026-10-18, "a skip with a note when no reviewer of another family is available. Approved."): the push goes out without a session review, with a note
5. stated (coordinator): "Docs: the build guide explains the loop."
````
