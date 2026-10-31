# rahulnakmol/agentic-ayudha#1173: Approve to build is an inbox card, with its stories, budget, agents, milestones, and high-risk entries

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `76015b9e42f6f31711a0dabbbe7009f4f5e66b80c29e2fa54315309e92d2f622` |
| Completion Goal hash | `82eba705be56965bb244c6465d868d442b4434ed0bb16c173846d808a6de3e52` |
| Approved to build | #1170 approval 2 by @rahulnakmol |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1173/ready.md` |

Completion Goal at Ready:

1. [recording] state: the intent, spec, and tech baselines of #889 are approved and it has no approval; expected: an Approve to build card shows its stories and Bugs, the budget (stories × `[budget] per_story_usd`), agents, the milestones, and its high-risk entries by story
2. [api-trace] state: he approves the card in the session where he approved the baselines; expected: no second device code, and specs/889/approved.json and missions/889/plan.json are written as `aayu ready 889` writes them
3. [api-trace] state: he sets agents to 10 on the card; expected: approved.json has agents 10
4. [api-trace] state: a story of #889 changed after the card was read; expected: nothing is approved, and the card says `The stories of #889 changed while you read it.`
5. [api-trace] state: the tech baseline of #889 is not approved; expected: the inbox shows no Approve to build card for #889, only its Baseline cards

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want Approve to build to be a card in the same morning session as the baselines, so that one code covers the whole Feature.

### Completion Goal

1. [recording] state: the intent, spec, and tech baselines of #889 are approved and it has no approval; expected: an Approve to build card shows its stories and Bugs, the budget (stories × `[budget] per_story_usd`), agents, the milestones, and its high-risk entries by story
2. [api-trace] state: he approves the card in the session where he approved the baselines; expected: no second device code, and specs/889/approved.json and missions/889/plan.json are written as `aayu ready 889` writes them
3. [api-trace] state: he sets agents to 10 on the card; expected: approved.json has agents 10
4. [api-trace] state: a story of #889 changed after the card was read; expected: nothing is approved, and the card says `The stories of #889 changed while you read it.`
5. [api-trace] state: the tech baseline of #889 is not approved; expected: the inbox shows no Approve to build card for #889, only its Baseline cards

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Approve three baselines and the Feature in one session; change agents; repeat with a story that changed and with a part not approved.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, a fake clock, and a fake device flow; never production

### Links

depends-on #1172

### Checkpoints

1. The card shows what Approve to build approves (criteria 1, 5)
2. Approving it in the baselines' session writes the approval and the plan with one code (criteria 2, 3, 4)

### Assumptions

#### Notes

The card runs `approveToBuild` (internal/cli/readyapprove.go) unchanged. The high-risk entries come from #1029; until it merges the card shows none and says so.

#### Review guidance

- Check: criterion 2: the record is the same as the command's
- Check: criterion 4: the stories are read again after the code
- Ignore: the order of the lines on the card
- Risk: an approval written for stories the person did not see

#### Behaviours

1. says: Approve to build in the inbox approves exactly what the card shows; rule: never; source: criteria 2, 4
2. says: A Feature with a part not approved has no Approve to build card; rule: never; source: criteria 5

### Provenance

Request: "I start my day by answering a series of questionnaires about the things a human needs to review and act upon … At the end of the day … I see that what I agreed upon in the morning has been implemented, delivered, and proven" (@rahulnakmol, 2026-10-30, Decision #1168)
1. stated (request): approval in the morning session
2. stated (request): one code covers the Feature
3. default (scope): agents set on the card, as `--agents` does
4. default (scope): the same refusal as the command
5. default (scope): baselines come first, as `prepare` requires
````
