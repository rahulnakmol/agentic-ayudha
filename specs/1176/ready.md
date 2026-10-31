# rahulnakmol/agentic-ayudha#1176: The evening review: each story agreed this morning, with each criterion next to its evidence

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `948f84fe551a347acaf5f8ff27ca04c58faec2bc978a4015fb9b073a5d47d583` |
| Completion Goal hash | `f053138cb555831e0aec9bc614300c30ffc290d020809ef3a509ab6491ab72a7` |
| Approved to build | #1170 approval 2 by @rahulnakmol |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1176/ready.md` |

Completion Goal at Ready:

1. [recording] state: #889's MS1 started at 09:00, and #1010 and #1011 merged with proof packs; expected: at the end of `[time] business_hours`, `aayu inbox` shows one Evening card per story, with each criterion, its evidence kind, and its evidence from the pack: the image where the terminal shows images, else its path and link
2. [api-trace] state: he accepts #1010; expected: missions/889/reviews.jsonl gains a line with his login, the time, #1010, and accepted
3. [api-trace] state: he comments on #1011 `the page names the wrong person`; expected: the comment is posted on #1011, and the mission's feedback pass reads it as one input
4. [api-trace] state: #1104 was agreed this morning and did not merge; expected: its card says `not merged:` with the mission's reason and the next step
5. [api-trace] state: an agent lane runs `aayu inbox --list --json` in the evening; expected: it lists the Evening cards, with no accept action

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want an evening card for each story I agreed this morning, with each criterion next to its evidence, so that I see what was built and accept it or say what is wrong.

### Completion Goal

1. [recording] state: #889's MS1 started at 09:00, and #1010 and #1011 merged with proof packs; expected: at the end of `[time] business_hours`, `aayu inbox` shows one Evening card per story, with each criterion, its evidence kind, and its evidence from the pack: the image where the terminal shows images, else its path and link
2. [api-trace] state: he accepts #1010; expected: missions/889/reviews.jsonl gains a line with his login, the time, #1010, and accepted
3. [api-trace] state: he comments on #1011 `the page names the wrong person`; expected: the comment is posted on #1011, and the mission's feedback pass reads it as one input
4. [api-trace] state: #1104 was agreed this morning and did not merge; expected: its card says `not merged:` with the mission's reason and the next step
5. [api-trace] state: an agent lane runs `aayu inbox --list --json` in the evening; expected: it lists the Evening cards, with no accept action

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Start a milestone in the morning, merge two stories with packs, leave one unmerged, then open the evening inbox and accept one and comment on another.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, a fake clock, and a fake device flow; never production

### Checkpoints

1. Evening cards show each agreed story with its evidence (criteria 1, 4)
2. Accepting or commenting records it (criteria 2, 3)
3. An agent lane cannot accept (criteria 5)

### Assumptions

#### Notes

The evidence is the proof pack on the ledger (`proof/<n>/<sha>/`), never a comment. The feedback pass is #1014 (in #889); until it merges, the comment stays on the issue.

#### Review guidance

- Check: criterion 1: every criterion of the story is shown, including those with no evidence
- Check: criterion 2: the record is on the ledger
- Ignore: how an image is drawn in each terminal
- Risk: a card that shows evidence from a different head than the merged one

#### Behaviours

1. says: Each story agreed in the morning is shown in the evening with its evidence; rule: limit; source: criteria 1, 4
2. says: Only a person accepts a story in the evening review; rule: never; source: criteria 5

### Provenance

Request: "I start my day by answering a series of questionnaires about the things a human needs to review and act upon … At the end of the day … I see that what I agreed upon in the morning has been implemented, delivered, and proven" (@rahulnakmol, 2026-10-30, Decision #1168)
1. stated (request): "I can even create screenshots and see evidence of everything"
2. stated (request): "see that something has been delivered as intended"
3. default (scope): a comment is feedback, as #1014 turns it into a story
4. stated (request): what did not land is said plainly
5. default (scope): agents read, never accept
````
