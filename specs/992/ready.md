# rahulnakmol/agentic-ayudha#992: Each baseline part pins what it owns: the spec's stories, the technical design, and the UX stages

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `eeefd14c22ef65d5820f621fc10f9e206135f3f0415e202b45f656aed9aaa79f` |
| Completion Goal hash | `13b2adcc3f4736017b796436c47fe8015af8f13ba1ac211477f77bde8a5476cf` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/992/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu ready 874 --part spec --request` with 5 child stories under #874; expected: the draft has one pin per story with its number, title, `workitems.CompletionGoalHash`, effort label, layer labels, and depends-on numbers, and one pin for the set of story numbers
2. [api-trace] state: two `aayu ready 874 --part spec --request --dry-run --json` runs, before and after an edit to only the Assumptions of one child story; expected: the two pin lists are byte for byte equal
3. [api-trace] state: `aayu ready 874 --part tech --request` with no `specs/874/design.json` on the ledger; expected: exit 4 with "The technical design of #874 has no design.json yet. Vishwa writes it first: aayu plan 874 --draft <file> --create"
4. [api-trace] state: `--part tech --request` for a Feature whose footprint has L05 and 1 comment with the marker `<!-- aayudha:chart kind=` (internal/chart/chart.go:153); expected: the draft pins `specs/<f>/design.json` by the SHA-256 of its bytes and the chart comment by the SHA-256 of its body
5. [api-trace] state: `aayu ready 874 --part ux --request` where no Expected path matches an L01 glob; expected: exit 4 with "UX design: not needed (no UI layer in the footprint)"
6. [api-trace] state: `--part ux --request` for a Feature with L01, the manifests `design/<f>/mockup/manifest.json` and `design/<f>/prototype/manifest.json` on the ledger, and an `early-build` story with an open PR; expected: the draft pins both manifests by the SHA-256 of their bytes and the PR by its head SHA

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want each part of a Feature to pin exactly what it owns, so that a wording edit is not drift and a real change is.

### Completion Goal

1. [api-trace] state: `aayu ready 874 --part spec --request` with 5 child stories under #874; expected: the draft has one pin per story with its number, title, `workitems.CompletionGoalHash`, effort label, layer labels, and depends-on numbers, and one pin for the set of story numbers
2. [api-trace] state: two `aayu ready 874 --part spec --request --dry-run --json` runs, before and after an edit to only the Assumptions of one child story; expected: the two pin lists are byte for byte equal
3. [api-trace] state: `aayu ready 874 --part tech --request` with no `specs/874/design.json` on the ledger; expected: exit 4 with "The technical design of #874 has no design.json yet. Vishwa writes it first: aayu plan 874 --draft <file> --create"
4. [api-trace] state: `--part tech --request` for a Feature whose footprint has L05 and 1 comment with the marker `<!-- aayudha:chart kind=` (internal/chart/chart.go:153); expected: the draft pins `specs/<f>/design.json` by the SHA-256 of its bytes and the chart comment by the SHA-256 of its body
5. [api-trace] state: `aayu ready 874 --part ux --request` where no Expected path matches an L01 glob; expected: exit 4 with "UX design: not needed (no UI layer in the footprint)"
6. [api-trace] state: `--part ux --request` for a Feature with L01, the manifests `design/<f>/mockup/manifest.json` and `design/<f>/prototype/manifest.json` on the ledger, and an `early-build` story with an open PR; expected: the draft pins both manifests by the SHA-256 of their bytes and the PR by its head SHA

### Measure

Pins whose hash changes when only text outside the pinned fields changes.

### Threshold

0, over the seeded runs of criteria 1 and 2.

### Scenario

An agent drafts the spec, the technical design, and the UX design of two seeded Features: one with a UI layer and one without.

### Environment

The fake GitHub of internal/github/fake, with its device flow, and a local ledger branch; synthetic issues only, never a production repository.

### Links

depends-on #990
depends-on #991

### Checkpoints

1. Spec pins come from each child story's pinned fields (criteria 1, 2)
2. Technical-design pins come from design.json and chart comments (criteria 3, 4)
3. UX pins come from the stage manifests and the early-build head (criteria 5, 6)

### Assumptions

- The part table and its owners are the core design's (aayu core v13, "G5 to G8" table): intent and spec Arjun, tech Vishwa, ux Hanu.
- Child stories come from `SubIssue.ListByIssue`, as internal/cli/chart.go:108 reads them.

#### Review guidance (Q-H: here until slice --update, #887 story 2, moves it)

check: 1 spec pins use `workitems.CompletionGoalHash`, never `workitems.BodyHash`
check: 6 manifests are pinned from the ledger bytes (`design.ManifestPath`), not from the working tree
risk: internal/ready/pins.go a pin over the whole body makes every wording edit a drift
ignore: category docs

#### Behaviours (Q-H: here until slice --update, #887 story 2, moves it)

1. says: "A spec baseline pins each story's Completion Goal, title, effort, layers, and dependencies, and not the rest of its body."; rule: capability; criteria 1, 2; source: core design S5 (spec pins)
2. says: "A Feature with no UI layer needs no UX baseline."; rule: capability; criteria 5; source: Q16; core design part table (ux only with L01)

### Open questions

none
````
