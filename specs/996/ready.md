# rahulnakmol/agentic-ayudha#996: A story is Ready only while its Feature's approval covers it

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `a7741895dc350aba395eae85e1a6cdeb4be89e760f4a4fd4951ee19a302a67bc` |
| Completion Goal hash | `5cc9c5bd31991a7b690d265cfe353ad9fb52266d489b543c6f659152ba3bf599` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/996/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: Story #857 has no parent Feature, and Arjun runs `aayu ready 857`; expected: refused with exit 4: "#857 has no parent Feature. A Story is Ready only under a Feature approved to build (Decision #861)."
2. [api-trace] state: Story #853 under Feature #840, which has no approved.json on the ledger; expected: refused: "Feature #840 is not approved to build. A person runs aayu ready 840."
3. [api-trace] state: #840 was approved with technical design v1, and technical design v2 was approved later (or the body of #840 changed); expected: `aayu ready 853` is refused: "#840 changed after Approve to build: technical design v1 to v2. Its stories wait until a person approves it again."
4. [api-trace] state: after approval 1 of #840, the effort of #853 changed from S to M, and Story #858 was added under #840; expected: `aayu ready 853` is refused: "#853 changed after Approve to build (effort S to M). A person acknowledges it: aayu ready 840 --ack 853"; `aayu ready 858` is refused: "#858 was added after Approve to build. A person acknowledges it: aayu ready 840 --ack 858"
5. [api-trace] state: #853 is covered by approval 1 of #840 and only its Assumptions text changed; expected: Arjun makes #853 Ready as today; the Ready snapshot has the row "Approved to build | #840 approval 1 by @rahulnakmol"
6. [api-trace] state: Story #852 has specs/840/approved-early-852.json and #840 has no full approval; expected: `aayu ready 852` passes; `aayu ready 853` is still refused as in criterion 2
7. [api-trace] state: Story #700 was made Ready before this rule, has no parent Feature, and gets a new label, so the aayudha-ready-guard workflow runs; expected: the guard keeps status:ready on #700, and `aayu build --next` still treats #700 as Ready

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want aayu ready <story> to refuse a story that my Approve to build does not cover, so that Arjun makes Ready only what I agreed.

### Completion Goal

1. [api-trace] state: Story #857 has no parent Feature, and Arjun runs `aayu ready 857`; expected: refused with exit 4: "#857 has no parent Feature. A Story is Ready only under a Feature approved to build (Decision #861)."
2. [api-trace] state: Story #853 under Feature #840, which has no approved.json on the ledger; expected: refused: "Feature #840 is not approved to build. A person runs aayu ready 840."
3. [api-trace] state: #840 was approved with technical design v1, and technical design v2 was approved later (or the body of #840 changed); expected: `aayu ready 853` is refused: "#840 changed after Approve to build: technical design v1 to v2. Its stories wait until a person approves it again."
4. [api-trace] state: after approval 1 of #840, the effort of #853 changed from S to M, and Story #858 was added under #840; expected: `aayu ready 853` is refused: "#853 changed after Approve to build (effort S to M). A person acknowledges it: aayu ready 840 --ack 853"; `aayu ready 858` is refused: "#858 was added after Approve to build. A person acknowledges it: aayu ready 840 --ack 858"
5. [api-trace] state: #853 is covered by approval 1 of #840 and only its Assumptions text changed; expected: Arjun makes #853 Ready as today; the Ready snapshot has the row "Approved to build | #840 approval 1 by @rahulnakmol"
6. [api-trace] state: Story #852 has specs/840/approved-early-852.json and #840 has no full approval; expected: `aayu ready 852` passes; `aayu ready 853` is still refused as in criterion 2
7. [api-trace] state: Story #700 was made Ready before this rule, has no parent Feature, and gets a new label, so the aayudha-ready-guard workflow runs; expected: the guard keeps status:ready on #700, and `aayu build --next` still treats #700 as Ready

### Measure

Completion Goal criteria observed

### Threshold

all 7 criteria

### Scenario

Run aayu ready on stories of a synthetic Feature in each approval state: no parent, not approved, drifted, story changed or added, covered, early build, and an older Ready story.

### Environment

local checkout with the GitHub fake and a synthetic ledger (internal/github/fake); never production

### Assumptions

#### Review guidance

- Check: Criterion 7: the new Story rules run in `aayu ready <story>` only, never in the guard or in the build pick (F4).
- Check: Criterion 5: a wording edit outside the Completion Goal, effort, and title needs no acknowledgment (#903).
- Risk: internal/ready/: the coverage check.
- Risk: internal/workitems/schema.go: a Story must not get ParentAt, or the guard strips older Ready stories.
- Ignore: category docs
- Ignore: settled #861 (every Story needs an approved Feature)

#### Behaviours

1. says: Only a person's approval or acknowledgment, at the story's current Completion Goal, effort, and title, lets Arjun make it Ready; rule: authority; source: criteria 2, 4, 5
2. says: A rule added at Ready never removes a Ready that was granted before it; rule: never; source: criteria 7

### Links

depends-on #995
````
