# rahulnakmol/agentic-ayudha#1012: A stateless wake on GitHub events, and a local Kanha for Codex, Copilot, and OpenCode

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `0f93de0b81cc98821820bc970e4d6a31e77fa21b3b958e500f5fa17f0017ff5c` |
| Completion Goal hash | `3220959c80d23c2bd7f3239ac01cc2457c8efed5b00ec2e51d49ff2b9f4309c3` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1012/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the J3/K2 probe: a Claude scheduled task and a Cursor Automation fire on `Pull request merged` for a story of #889, and run `aayu mission wake 889`; expected: the session reads missions/889/plan.json, starts the next story (`claude --cloud`, or a Cursor cloud agent) with no person's token, and its commits carry `Aayudha-Host: claude` or `Aayudha-Host: cursor`; if (a) the start or (b) the sign-in fails, the wake only records the next stories in the plan, and the local or Amp Kanha starts them
2. [api-trace] state: the Kanha of #889 is a local Codex session `s-123`, and story #40 merges; expected: aayu runs `codex exec resume s-123 "merged #40 (PR #41): run aayu mission next 889"` on his machine
3. [api-trace] state: the Kanha of #889 is a local Copilot session `p-456`, and story #63 blocks; expected: aayu runs `copilot -p --resume p-456 "blocked #63: spec unclear: run aayu mission next 889"`
4. [api-trace] state: the mission's recorded tool is copilot; expected: Start starts a local Copilot session with `copilot -p` and resumes it with `copilot -p --resume <id>`; it never makes a Copilot cloud task the Kanha, because a cloud task takes no follow-up
5. [api-trace] state: the Kanha of #889 is a local OpenCode session `o-789`, and story #33's ask is answered; expected: aayu runs `opencode run --session o-789 "answered: #33 resumes"`, and #889's story subthreads run as local OpenCode sessions
6. [api-trace] state: no `[mission] check_minutes` in any config, and an event arrived 4 minutes ago; expected: the local Kanha takes the event at its next check, 5 minutes after the last one, or at once when @rahulnakmol opens `aayu inbox`
7. [api-trace] state: his machine is off when story #40 merges; expected: the plan view says `#889 paused: its Kanha runs on rahulnakmol@laptop, which is off; it resumes at the next check`, and no story of #889 starts until then
8. [api-trace] state: the mission's recorded tool is cursor, and this machine has no Cursor API key; expected: the fallback is a local Cursor chat `c-456`, resumed with `agent -p --resume=c-456 "<event>"`, and the plan view says `#889's Kanha is local: it pauses while rahulnakmol@laptop is off`

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want a Kanha on Codex, Copilot, or OpenCode too, so that every host I use can steer a Feature.

### Completion Goal

1. [api-trace] state: the J3/K2 probe: a Claude scheduled task and a Cursor Automation fire on `Pull request merged` for a story of #889, and run `aayu mission wake 889`; expected: the session reads missions/889/plan.json, starts the next story (`claude --cloud`, or a Cursor cloud agent) with no person's token, and its commits carry `Aayudha-Host: claude` or `Aayudha-Host: cursor`; if (a) the start or (b) the sign-in fails, the wake only records the next stories in the plan, and the local or Amp Kanha starts them
2. [api-trace] state: the Kanha of #889 is a local Codex session `s-123`, and story #40 merges; expected: aayu runs `codex exec resume s-123 "merged #40 (PR #41): run aayu mission next 889"` on his machine
3. [api-trace] state: the Kanha of #889 is a local Copilot session `p-456`, and story #63 blocks; expected: aayu runs `copilot -p --resume p-456 "blocked #63: spec unclear: run aayu mission next 889"`
4. [api-trace] state: the mission's recorded tool is copilot; expected: Start starts a local Copilot session with `copilot -p` and resumes it with `copilot -p --resume <id>`; it never makes a Copilot cloud task the Kanha, because a cloud task takes no follow-up
5. [api-trace] state: the Kanha of #889 is a local OpenCode session `o-789`, and story #33's ask is answered; expected: aayu runs `opencode run --session o-789 "answered: #33 resumes"`, and #889's story subthreads run as local OpenCode sessions
6. [api-trace] state: no `[mission] check_minutes` in any config, and an event arrived 4 minutes ago; expected: the local Kanha takes the event at its next check, 5 minutes after the last one, or at once when @rahulnakmol opens `aayu inbox`
7. [api-trace] state: his machine is off when story #40 merges; expected: the plan view says `#889 paused: its Kanha runs on rahulnakmol@laptop, which is off; it resumes at the next check`, and no story of #889 starts until then
8. [api-trace] state: the mission's recorded tool is cursor, and this machine has no Cursor API key; expected: the fallback is a local Cursor chat `c-456`, resumed with `agent -p --resume=c-456 "<event>"`, and the plan view says `#889's Kanha is local: it pauses while rahulnakmol@laptop is off`

### Measure

Events for a local Kanha that waited more than one check while the machine was on.

### Threshold

0

### Scenario

@rahulnakmol works in Codex. A story merges at 14:02, and his local Kanha starts the next cloud story by 14:07. In OpenCode the stories run locally too.

### Environment

go test of internal/delegate with stub codex, copilot, opencode, and agent CLIs, and of internal/config; never production.

### Links

depends-on #1004

### Assumptions

#### Notes

Depends on, outside this slice: R9 per-person, per-repository tool entry (Tatkal story), #917.

Criterion 1 is the J3/K2 probe; it runs when MS3 opens. If it fails, the local Kanha design below stays. The cloud wake applies where the setup's cloud environment is set up, and the local Kanha applies otherwise; it is not behind `--cloud` (@rahulnakmol, 2026-10-23: planning on the local CLI or a desktop app, the build loop in cloud threads). The commands `codex exec resume <session> <event>`, `copilot -p --resume`, `opencode run --session <id> <event>`, and the Cursor fallback `agent -p --resume=<chat> <event>` are from the cloud-agents page and #893; the plan checks each against its CLI. Copilot story threads start through #917's Copilot receivers (C3, C4). The Cursor adapter gaps (agentId, 409 agent_busy, /usage, an ARCHIVED agent) are K3: one Standalone story, or folded in here (then this story is L). OpenCode has no hosted cloud agent, so its story subthreads are local only (#893 story F). internal/config/ is in `[autonomy] always_person`, so @rahulnakmol ships this PR. #911 reuses these resume adapters, because each loop's subthread is resumed, never new.

#### Review guidance

check: The resume commands take the event text only; issue text never reaches them. A Codex cloud task takes no CLI follow-up (receivers.go:400-401), so Codex story fixes stay a new task on the PR branch, as today. Cursor needs no local Kanha when it has an API key: its cloud agent keeps its conversation (receivers.go:170-196, K1)
ignore: The wording of the event text.
risk: internal/config/ is in the narrow set (.aayudha/config.toml:83). A machine that stays off pauses its Features with no page.

#### Behaviours

1. [capability] says: One wake command, aayu mission wake, serves every event-driven host; a failed probe falls back to record-only; source: J3 and K2, accepted 2026-10-23; criteria: 1
2. [never] says: A Copilot cloud task never coordinates a mission; source: Y9/K1 A, 2026-10-23; criteria: 4
3. [limit] says: A local Kanha takes each event at its next check, or at once when he opens the inbox; source: decision Y9 recommendation (all five hosts), relayed by the design thread 2026-10-22; criteria: 6
4. [never] says: While the Kanha's machine is off, its Feature starts no story; source: decision Y9 recommendation (all five hosts), relayed by the design thread 2026-10-22; criteria: 7
````
