# rahulnakmol/agentic-ayudha#1013: Kanha's lock at mission start: the bodies and the size, risk, and priority labels of the Feature, its stories, and its Bugs become read-only

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `df7433d1c991bf30fa65cba7b4ab329260f3538ed7d37ac391b3f99034443556` |
| Completion Goal hash | `d8eb0349ee413a9284e6638abdd821409eed83f60fb266935ca005a1bb07629e` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1013/ready.md` |

Completion Goal at Ready:

1. [screenshot] state: #889 is approved to build, its 7 stories are Ready, and it has 1 open Bug, and @rahulnakmol opens `Start the mission of #889` in aayu inbox; expected: the card says `Starting the mission makes 9 issues read-only until its last milestone:` with #889, its stories, and its open Bugs, then `Locked  each body, and its effort:, risk: and priority: labels`, `Open    comments and status: labels`, and `Ends    at the last milestone of the last mission (MS3)`; it says `It runs in Cursor, the tool you set for this repository.` and `up to 5 stories at once`, offers no tool choice, and its choices are `1 Start the mission` (presence check), `2 Not now: nothing is locked`, and `3 Read the issues first`
2. [api-trace] state: he presses `1 Start the mission` (it runs `aayu mission start 889`) and passes the presence check; expected: missions/889/plan.json on the ledger gains `lock: {state: "active", confirmed_by: "rahulnakmol", started_at: "<RFC 3339>", ended_at: null, end_reason: null, issues: [{number: 20, body_sha256: "<hash>", labels: {effort: "effort:S", risk: "risk:medium", priority: "priority:P1"}}, …]}`
3. [api-trace] state: he picks `2 Not now`; expected: no mission starts, nothing is locked, and the card stays in the inbox
4. [api-trace] state: an agent runs an aayu command that changes #20's body or its `priority:` label while #889 runs; expected: aayu refuses before the request leaves the client with `#20 is locked by mission #889 until it ends`
5. [screenshot] state: @alice edits criterion 3 of #20's body on GitHub at 10:42 while #889 runs; expected: at the next wake or `aayu mission next 889`, aayu sees that #20's body hash differs from the lock and never rewrites the body; `aayu mission next 889` shows `waits #20: changed while locked`; #20 gets one comment that starts `<!-- aayudha:lock -->` and says `This body changed while the mission of #889 holds it: criterion 3, edited by @alice at 10:42. aayu did not change it back. #20 waits until a person keeps the edit in the inbox, or the mission ends.`; and an inbox card `#20 changed while locked by the mission of #889` offers `1 Keep the edit: #20 is planned again from its new body`, `2 Leave #20 waiting`, and `3 Open #20 on GitHub to undo the edit yourself`, with no recommendation
6. [api-trace] state: a person comments on #20, and #20's `status:` label changes, while #889 runs; expected: both go through, because the lock covers only bodies and the `effort:`, `risk:`, and `priority:` labels
7. [api-trace] state: the last milestone of #889's last mission passes its gate; expected: `lock.state` becomes `ended` and `lock.ended_at` is set
8. [api-trace] state: mission 2 of #889 starts after mission 1 ended; expected: no new start card: the lock that he confirmed at mission 1 lists every issue of #889 and stays `active` between missions
9. [api-trace] state: @rahulnakmol picks `1 Keep the edit` on #20's card; expected: the lock takes #20's new body hash, `controls` in plan.json records the keep by him, and `aayu mission next 889` lists #20 again by the usual rules

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want the work a mission builds to stay fixed while it runs, so that no story changes under a running agent.

### Completion Goal

1. [screenshot] state: #889 is approved to build, its 7 stories are Ready, and it has 1 open Bug, and @rahulnakmol opens `Start the mission of #889` in aayu inbox; expected: the card says `Starting the mission makes 9 issues read-only until its last milestone:` with #889, its stories, and its open Bugs, then `Locked  each body, and its effort:, risk: and priority: labels`, `Open    comments and status: labels`, and `Ends    at the last milestone of the last mission (MS3)`; it says `It runs in Cursor, the tool you set for this repository.` and `up to 5 stories at once`, offers no tool choice, and its choices are `1 Start the mission` (presence check), `2 Not now: nothing is locked`, and `3 Read the issues first`
2. [api-trace] state: he presses `1 Start the mission` (it runs `aayu mission start 889`) and passes the presence check; expected: missions/889/plan.json on the ledger gains `lock: {state: "active", confirmed_by: "rahulnakmol", started_at: "<RFC 3339>", ended_at: null, end_reason: null, issues: [{number: 20, body_sha256: "<hash>", labels: {effort: "effort:S", risk: "risk:medium", priority: "priority:P1"}}, …]}`
3. [api-trace] state: he picks `2 Not now`; expected: no mission starts, nothing is locked, and the card stays in the inbox
4. [api-trace] state: an agent runs an aayu command that changes #20's body or its `priority:` label while #889 runs; expected: aayu refuses before the request leaves the client with `#20 is locked by mission #889 until it ends`
5. [screenshot] state: @alice edits criterion 3 of #20's body on GitHub at 10:42 while #889 runs; expected: at the next wake or `aayu mission next 889`, aayu sees that #20's body hash differs from the lock and never rewrites the body; `aayu mission next 889` shows `waits #20: changed while locked`; #20 gets one comment that starts `<!-- aayudha:lock -->` and says `This body changed while the mission of #889 holds it: criterion 3, edited by @alice at 10:42. aayu did not change it back. #20 waits until a person keeps the edit in the inbox, or the mission ends.`; and an inbox card `#20 changed while locked by the mission of #889` offers `1 Keep the edit: #20 is planned again from its new body`, `2 Leave #20 waiting`, and `3 Open #20 on GitHub to undo the edit yourself`, with no recommendation
6. [api-trace] state: a person comments on #20, and #20's `status:` label changes, while #889 runs; expected: both go through, because the lock covers only bodies and the `effort:`, `risk:`, and `priority:` labels
7. [api-trace] state: the last milestone of #889's last mission passes its gate; expected: `lock.state` becomes `ended` and `lock.ended_at` is set
8. [api-trace] state: mission 2 of #889 starts after mission 1 ended; expected: no new start card: the lock that he confirmed at mission 1 lists every issue of #889 and stays `active` between missions
9. [api-trace] state: @rahulnakmol picks `1 Keep the edit` on #20's card; expected: the lock takes #20's new body hash, `controls` in plan.json records the keep by him, and `aayu mission next 889` lists #20 again by the usual rules

### Measure

Locked bodies or labels that an aayu write changed while a mission ran.

### Threshold

0

### Scenario

@rahulnakmol confirms #889's start card once. The stories build. A person's edit to one story holds only that story, and nothing rewrites it.

### Environment

go test of internal/cli with a fake GitHub and ledger; never production.

### Links

depends-on #997

### Assumptions

#### Notes

Depends on, outside this slice: #974.

Decision #974 (amends #915) is accepted. GitHub has no read-only issue body, so aayu refuses its own writes and holds a story that a person edited. Comments stay open, because the feedback pass (ST19) reads them. `--end` lifts the lock (ST16). The start card's view is #895's. #879 S7 (`aayu triage --decide story --feature <n>`) refuses a Feature whose `lock.state` is `active`. With several missions (X1 A), the lock is the Feature's: it starts at the first mission and ends when the last one ends or at `--end`, so #879 S7's `lock.state` check still reads one field. The words come from 0.4.0 UX baseline v44, approved by @rahulnakmol: terminal frames tui-start and tui-locked, web card j-inbox-card-889, and GitHub comment 889-locked. The card itself is drawn by the inbox Features (#895, #975); this story owns its words and its effects.

#### Review guidance

check: The refusal wraps the transport of the CLI's one GitHub client for writes, `Env.newGitHub` (internal/cli/github.go:14-29), so it covers every aayu write without a change to internal/github. A GitHub edit cannot be blocked: the mission compares each locked body with its `body_sha256` on every wake and `aayu mission next`, and holds the story. The Ready guard (`Ready.Guard`, internal/ready/ready.go:247, #877's) is not changed; it already strips `status:ready` when a Ready body drifts.
ignore: The card's layout (#895, #975); the words are fixed above.
risk: A lock that never ends blocks a person's later edits. The lock ends at the mission's end or at `--end` (ST16).

#### Behaviours

1. [authority] says: A lock starts only when a person confirms the start card; source: Decision #974 (amends #915); criteria: 1, 2, 3, 8
2. [never] says: While a mission runs, no aayu write changes a locked body or label; source: Decision #974 (amends #915); criteria: 4
3. [never] says: aayu never rewrites a body that a person edited; it holds the story; source: Decision #974 (amends #915); delivered bodies are never edited; criteria: 5
4. [authority] says: Only a person keeps an edit made while locked; source: 0.4.0 UX baseline v44, approved by @rahulnakmol (locked-edit card); criteria: 9
````
