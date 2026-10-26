# rahulnakmol/agentic-ayudha#1004: Start on the mission start card starts the mission's Kanha on the tool recorded in the plan

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `080972918a74f749c228109b3d399e41f4f1d5d86636864ace8980937fb52591` |
| Completion Goal hash | `9df3f4e99a25127b2bd81d0b9e545dad3ff9d9b821204a08517c3df78047f6bf` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1004/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: @rahulnakmol starts #889's mission (`aayu mission start 889`) from the thread that drafted its intent, on amp, and his tool for this repository is amp; expected: specs/889/approved.json has `parent` with that thread's host, session, and URL, and that thread gets the Kanha brief for #889
2. [api-trace] state: the drafting thread is not known or is archived, and the starter's tool for this repository is claude; expected: aayu starts a new Claude Code session with the Kanha brief and records it as `parent`
3. [api-trace] state: the starter's tool for this repository is codex, copilot, or opencode; expected: Start starts a local Kanha on the starter's machine through ST13, records it as `parent` with mode local and login@machine, and says `the Kanha of #889 runs on this machine; while it is off, #889 pauses at the next merge`
4. [api-trace] state: the starter's tool for this repository is cursor; expected: Start creates one Cursor cloud agent with the Kanha brief and records it as `parent` with mode cloud; each later event is one run on that agent (POST /v1/agents/{id}/runs)
5. [api-trace] state: the Kanha starts story #60 of #889; expected: delegate/60/state.json has `feature: 889` and the Kanha's session and URL
6. [api-trace] state: mission #889's recorded tool is cursor, and a story's brief asks for codex; expected: the Kanha starts the story on cursor; a mission never mixes agent tools
7. [api-trace] state: @rahulnakmol (tool cursor) starts #889's mission, and @bob (tool codex) starts #874's in the same repository; expected: #889's Kanha and its story threads run on Cursor, #874's on Codex through ST13, and each plan.json names its own tool
8. [api-trace] state: in Tatkal Feature #920, @bob (tool codex) starts story #930, and @rahulnakmol (tool cursor) starts story #931; expected: #930 runs on codex and #931 on cursor, missions/920/plan.json records each story's tool and starter, and #920's Kanha orders and tracks both and starts neither

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want one Start to begin one Kanha on the tool I set for this repository, so that each mission's stories start with no further step from me.

### Completion Goal

1. [api-trace] state: @rahulnakmol starts #889's mission (`aayu mission start 889`) from the thread that drafted its intent, on amp, and his tool for this repository is amp; expected: specs/889/approved.json has `parent` with that thread's host, session, and URL, and that thread gets the Kanha brief for #889
2. [api-trace] state: the drafting thread is not known or is archived, and the starter's tool for this repository is claude; expected: aayu starts a new Claude Code session with the Kanha brief and records it as `parent`
3. [api-trace] state: the starter's tool for this repository is codex, copilot, or opencode; expected: Start starts a local Kanha on the starter's machine through ST13, records it as `parent` with mode local and login@machine, and says `the Kanha of #889 runs on this machine; while it is off, #889 pauses at the next merge`
4. [api-trace] state: the starter's tool for this repository is cursor; expected: Start creates one Cursor cloud agent with the Kanha brief and records it as `parent` with mode cloud; each later event is one run on that agent (POST /v1/agents/{id}/runs)
5. [api-trace] state: the Kanha starts story #60 of #889; expected: delegate/60/state.json has `feature: 889` and the Kanha's session and URL
6. [api-trace] state: mission #889's recorded tool is cursor, and a story's brief asks for codex; expected: the Kanha starts the story on cursor; a mission never mixes agent tools
7. [api-trace] state: @rahulnakmol (tool cursor) starts #889's mission, and @bob (tool codex) starts #874's in the same repository; expected: #889's Kanha and its story threads run on Cursor, #874's on Codex through ST13, and each plan.json names its own tool
8. [api-trace] state: in Tatkal Feature #920, @bob (tool codex) starts story #930, and @rahulnakmol (tool cursor) starts story #931; expected: #930 runs on codex and #931 on cursor, missions/920/plan.json records each story's tool and starter, and #920's Kanha orders and tracks both and starts neither

### Measure

Approved Features with Ready stories and no Kanha, per 28 days.

### Threshold

0

### Scenario

@rahulnakmol approves #889 in this thread at 10:00. This thread becomes its Kanha and starts the first story at 10:01.

### Environment

go test of internal/cli and internal/delegate with a fake ledger and a stub adapter; never production.

### Links

depends-on #997
depends-on #998

### Assumptions

#### Notes

Depends on, outside this slice: #877 S1, #877 S2, #871, R9 per-person, per-repository tool entry (Tatkal story).

Needs #877 S1 and S2 (Approve to build writes approved.json, which Start requires; D1 A); `aayu mission start` is ST14's, and the start card's words are ST18's, #871 (the host and session source), and the R9 Tatkal story, which adds a per-repository tool entry to each person's user config (person-only, as `[hosts.cursor] plan` is: internal/config/defaults.toml:148-152); `aayu init` asks once, and to change tools a person runs setup again. ST14 records the tool and the person who started the mission in the plan; the Kanha, story threads, orbs, and teammates read the tool from there, never from a config, so two missions in one repository may run on different tools. Y9 and K1 are A (2026-10-23): Amp, Claude Code, and Cursor run a cloud Kanha; Codex, Copilot, and OpenCode run a local one (ST13).

#### Review guidance

check: aayu starts a Kanha only at a person's Start, after Approve to build, never on its own (0.4.0 UX baseline v44, approved by @rahulnakmol) (Decision #915 (M1), F1). The parent is information, never authority (#871). A Codex cloud task takes no follow-up from the CLI: aayu's follow-up starts a new task (receivers.go:400-401), so a Codex Kanha is local only. A Cursor cloud agent keeps its conversation across runs (POST /v1/agents/{id}/runs, receivers.go:170-196), so it can be a cloud Kanha (K1 A). The tool comes from setup, never from Approve to build or a story's brief (R9). The parent record and the start call take a scope (`feature` or `repo`) and a work key, not only a story number (`Handoff.Story`, receivers.go:19-26), so Kala (#911) uses the same code.
ignore: The Kanha brief's wording outside its rules.
risk: A Kanha on a closed thread drops every wake. Start is human-only, so the start must not move to a bot path.

#### Behaviours

1. [authority] says: Only a person's Start starts a Kanha; aayu never starts one on its own; source: Decision #915 (M1), F1; 0.4.0 UX baseline v44, approved by @rahulnakmol; criteria: 1, 2
2. [capability] says: Each story subthread names its Feature and its Kanha; source: core design E5; Decision #915 (M1), F1; criteria: 5
3. [never] says: A Feature mission's Kanha and all its story threads run on the tool recorded in its plan: the tool of the person who started it; source: R9, @rahulnakmol 2026-10-23; criteria: 6, 7
4. [authority] says: A Tatkal story runs on the tool of the person who starts it; the Tatkal Kanha only orders and tracks; source: R9 and the Tatkal rule, @rahulnakmol 2026-10-23; criteria: 8
````
