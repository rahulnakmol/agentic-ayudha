# rahulnakmol/agentic-ayudha#568: inspect's review converges within three rounds, and a defect the PR did not write is tagged pre-existing

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `f0c078a023eeadcad88a1c86b780cd4b6039943ac8efbf15b1da69fe3c1d7b33` |
| Completion Goal hash | `22342d016f13462aaacda8726549d6a495d365a2003cbc918d2253c513e96f2e` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/568/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a PR whose inspect state records 3 rounds and a new head with no reply; `aayu inspect <pr>` outside CI; expected: no review request is posted, the disposition is step-back, exit 4, and the next step says to reconsider the approach and that a person's `/aayu review again` reopens review
2. [api-trace] state: the same PR in the merge gate (`--ci`) with the reviewer's reply for the new head; expected: disposition step-back, exit 4, the reply is not judged, and the state still records 3 rounds
3. [api-trace] state: the same PR after a bot account's comment `/aayu review again`, then a person's; expected: the bot's comment reopens nothing; after the person's comment the request is posted and the reply is judged as round 4
4. [api-trace] state: a PR with one judged round at head H1 that kept one finding, and a new head H2; `aayu inspect <pr>` outside CI; expected: the request for H2 names H1, asks for a review of `git diff H1..H2` plus any P0 anywhere, and lists the round-1 finding under "do not repeat"
5. [api-trace] state: the round-2 reply repeats the round-1 finding reworded in the same file and class, and raises a P0 security finding similar to a round-1 finding; expected: the reworded repeat is dropped as `repeat`, counted, and not posted; the P0 security finding is kept
6. [api-trace] state: a stated-format reply whose finding carries `class` and `instances` in two files; expected: the inspect comment shows the class and every instance and asks the author to fix every instance
7. [api-trace] state: a reply with a P1 finding whose quote is an unchanged context line of a changed file, and Codex's own review with such a P1; expected: each finding is tagged pre-existing, the disposition stays merge, and the inspect comment lists them under Pre-existing
8. [api-trace] state: outside CI, a PR whose gate result holds two pre-existing findings, one a P0 security finding, in a checkout with the merge base; `aayu inspect <pr>` twice; expected: one Bug is filed through grill listing both findings with the commit `git blame` names for each line, `priority:P1`, a page for the P0, and the second run files no second Bug
9. [metric] state: a PR whose review passed on round 2; expected: the inspect state records rounds 2, and `aayu audit` shows rounds-to-pass for the PR

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the Engineering Manager, I want inspect's review to converge and to tell this PR's defects from older ones, so that an agent builder and an agent reviewer cannot loop for 27 rounds as they did on PR #471, and a PR is never held for a defect it did not write (Decision #544, points 1 and 2, option 1 each).

Point 1, convergence, enforced in the gate. A round is one reviewer reply judged on a distinct head. After round 3, inspect posts no new review request and its disposition is `step-back`: a fix disposition for the merge gate that asks the builder to reconsider the approach. Only a person's comment `/aayu review again` reopens review. From round 2 the request names the last reviewed head and asks for a review of `git diff <last>..<head>` only, plus any P0 anywhere, and lists every earlier finding with "do not repeat". The gate drops a finding that repeats an earlier round's finding (same file and class, or similar wording by the existing word-similarity test) as `repeat`, counted and never posted; a protected P0 is never dropped as a repeat. Each finding may carry a `class` and its `instances`, and the author is asked to fix every instance. Rounds-to-pass per PR is kept in the inspect state and shown by `aayu audit`. The build and ship skills act only on findings in aayu's inspect result, never on raw app comments.

Point 2, a Pre-existing tag. A finding whose quoted line is not an added or changed line of the PR's diff is tagged `pre-existing`; it never changes the disposition or blocks the PR. `aayu inspect <pr>`, run by the builder or a person, names the commit that introduced each such line with `git blame` at the merge base and files all of the PR's Pre-existing findings as one Bug through grill, deduplicated against open Bugs by finding ID, linked from the inspect comment; a protected P0 among them gives the Bug `priority:P1` and pages.

### Completion Goal

1. [api-trace] state: a PR whose inspect state records 3 rounds and a new head with no reply; `aayu inspect <pr>` outside CI; expected: no review request is posted, the disposition is step-back, exit 4, and the next step says to reconsider the approach and that a person's `/aayu review again` reopens review
2. [api-trace] state: the same PR in the merge gate (`--ci`) with the reviewer's reply for the new head; expected: disposition step-back, exit 4, the reply is not judged, and the state still records 3 rounds
3. [api-trace] state: the same PR after a bot account's comment `/aayu review again`, then a person's; expected: the bot's comment reopens nothing; after the person's comment the request is posted and the reply is judged as round 4
4. [api-trace] state: a PR with one judged round at head H1 that kept one finding, and a new head H2; `aayu inspect <pr>` outside CI; expected: the request for H2 names H1, asks for a review of `git diff H1..H2` plus any P0 anywhere, and lists the round-1 finding under "do not repeat"
5. [api-trace] state: the round-2 reply repeats the round-1 finding reworded in the same file and class, and raises a P0 security finding similar to a round-1 finding; expected: the reworded repeat is dropped as `repeat`, counted, and not posted; the P0 security finding is kept
6. [api-trace] state: a stated-format reply whose finding carries `class` and `instances` in two files; expected: the inspect comment shows the class and every instance and asks the author to fix every instance
7. [api-trace] state: a reply with a P1 finding whose quote is an unchanged context line of a changed file, and Codex's own review with such a P1; expected: each finding is tagged pre-existing, the disposition stays merge, and the inspect comment lists them under Pre-existing
8. [api-trace] state: outside CI, a PR whose gate result holds two pre-existing findings, one a P0 security finding, in a checkout with the merge base; `aayu inspect <pr>` twice; expected: one Bug is filed through grill listing both findings with the commit `git blame` names for each line, `priority:P1`, a page for the P0, and the second run files no second Bug
9. [metric] state: a PR whose review passed on round 2; expected: the inspect state records rounds 2, and `aayu audit` shows rounds-to-pass for the PR

### Measure

Completion Goal criteria observed

### Threshold

all 9 criteria

### Scenario

Run `aayu inspect` through the real CLI against the GitHub fake, outside CI and with `--ci`, with reviewer replies in the stated format and Codex's own format, across rounds on distinct heads, and read the exit code, the result, the posted comments, and the filed Bug; run `aayu audit` on the resulting ledger.

### Environment

local checkout and this repository's CI build job on a pull request; the GitHub fake (internal/github/fake); no live reviewer app

### Checkpoints

1. The round cap blocks the re-request and gives step-back, and only a person reopens it (criteria 1, 2, 3)
2. The request from round 2 is diff-only, lists earlier findings, and the gate drops repeats (criteria 4, 5)
3. Findings carry a class and instances (criteria 6)
4. Pre-existing findings are tagged, never block, and are filed as one Bug (criteria 7, 8)
5. Rounds-to-pass is recorded and shown (criteria 9)

### Assumptions

- The cap counts judged replies on distinct heads, whatever the disposition, so a merge at round 3 followed by another push needs a person's `/aayu review again`.
- A reopen gives three more rounds, counted from the person's comment.
- The model path (`reviewer = "model"`) gets the same cap and step-back in place of today's ErrCycles stop.
- Blame and the Bug are done outside CI, where a checkout exists; the gate only tags, because its inspect job checks out no code.
- Rounds-to-pass goes to `aayu audit`; retro and status read audit's flow and are updated where they print the cycle count.
- The founder's step stays theirs: turn off Codex's review on every push in its repository settings.

### Provenance

Request: "Build points 1+2 (convergence with a real round cap that blocks the re-request, plus the Pre-existing tag) as one story, then 4, 5, 3, 6 in that order; 7 stays deferred."
1. stated (Decision #544, point 1): "After round 3, inspect posts no new review request."
2. stated (Decision #544, point 1): "only a person's comment `/aayu review again` reopens review"
3. stated (Decision #544, point 1): "only a person's comment `/aayu review again` reopens review"
4. stated (Decision #544, point 1): "From round 2, the request names the last reviewed head and asks for a review of `git diff <last>..<head>` only, plus any P0 anywhere."
5. stated (Decision #544, point 1): "A protected P0 is never dropped as a repeat."
6. stated (Decision #544, point 1): "Each finding gets a `class` … and `instances`"
7. stated (Decision #544, point 2): "A Pre-existing finding never changes the disposition or blocks the PR."
8. stated (Decision #544, point 2): "All of a PR's Pre-existing findings go into one Bug, filed through `aayu grill bug`, deduplicated against open Bugs by `FindingID`"
9. stated (Decision #544, point 1): "Rounds-to-pass per PR goes into the inspect state, retro, `status` and `audit`"
````
