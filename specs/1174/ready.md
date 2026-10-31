# rahulnakmol/agentic-ayudha#1174: Bugs come last: a mission ends with a BUGS milestone, and a Bug filed during the mission joins it

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `3438c77281326c355b8ac7e1a3dd43095c208c6b8b172c21c419d363b022b97a` |
| Completion Goal hash | `913ca6c56a1246ae41c5ce741a3c83044b129a17346167d58580c0278600ae1f` |
| Approved to build | #1170 approval 2 by @rahulnakmol |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1174/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: #889 has open stories #20 and #21 and open Bugs #30 and #31, and `aayu ready 889` runs with no --milestones; expected: missions/889/plan.json has MS1 with #20 and #21, then BUGS with #30 and #31
2. [api-trace] state: --milestones names MS1 and MS2 and no Bug; expected: the plan adds BUGS with every open Bug, last
3. [api-trace] state: Bug #32 is filed under #889 while its mission runs; expected: the plan's BUGS milestone gains #32, and `aayu mission next 889` lists it after the stories of the milestones before
4. [api-trace] state: Bug #31 merged, and the ledger has no proof pack for its PR; expected: the BUGS gate fails with `#31 merged with no proof pack`
5. [api-trace] state: #889 has no open Bug; expected: the plan has no BUGS milestone

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want each Feature's Bugs fixed in a final BUGS milestone, so that a Feature ends with its Bugs merged with evidence.

### Completion Goal

1. [api-trace] state: #889 has open stories #20 and #21 and open Bugs #30 and #31, and `aayu ready 889` runs with no --milestones; expected: missions/889/plan.json has MS1 with #20 and #21, then BUGS with #30 and #31
2. [api-trace] state: --milestones names MS1 and MS2 and no Bug; expected: the plan adds BUGS with every open Bug, last
3. [api-trace] state: Bug #32 is filed under #889 while its mission runs; expected: the plan's BUGS milestone gains #32, and `aayu mission next 889` lists it after the stories of the milestones before
4. [api-trace] state: Bug #31 merged, and the ledger has no proof pack for its PR; expected: the BUGS gate fails with `#31 merged with no proof pack`
5. [api-trace] state: #889 has no open Bug; expected: the plan has no BUGS milestone

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Approve a Feature with stories and Bugs with and without --milestones, file a Bug during its mission, and gate BUGS with a Bug that has no pack.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, a fake clock, and a fake device flow; never production

### Checkpoints

1. Approve to build puts every open Bug last (criteria 1, 2, 5)
2. A Bug filed during the mission joins BUGS (criteria 3)
3. The BUGS gate needs a proof pack per Bug (criteria 4)

### Assumptions

#### Notes

A Bug needs no Approve to build (`storyCovered`, internal/cli/readycover.go), so joining BUGS needs no new approval. The lock (#1013) admits a new Bug as it admits comments.

#### Review guidance

- Check: criterion 3: a Bug filed during the mission is never left out of every milestone
- Check: criterion 4: the gate reads the ledger, not a comment
- Ignore: the milestone's name in messages
- Risk: a Bug that merges before the Feature's stories and is never gated

#### Behaviours

1. says: A Feature's Bugs are fixed last, in BUGS; rule: limit; source: criteria 1, 2, 3
2. says: A Bug counts as done in BUGS only with its proof pack; rule: never; source: criteria 4

### Provenance

Request: "I start my day by answering a series of questionnaires about the things a human needs to review and act upon … At the end of the day … I see that what I agreed upon in the morning has been implemented, delivered, and proven" (@rahulnakmol, 2026-10-30, Decision #1168)
1. stated (request): "bugs related to feature need to sequence at the end of each feature mission"
2. stated (request): the same with milestones named
3. stated (request): bugs found in the mission too
4. stated (request): "fixed and merged with evidence"
5. default (scope): no empty milestone
````
