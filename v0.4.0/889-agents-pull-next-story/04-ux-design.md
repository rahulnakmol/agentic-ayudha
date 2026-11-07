# UX design: Feature #889

Two people use this Feature: the person who approves, starts, stops, and extends a mission, and the agents (the Kanha
and the story subthreads) that run it. The delivered surface is the `aayu mission` verb, a few `aayu build` flags,
inbox cards, and the mission skill. Help text below is from `aayu mission --help` and `aayu build --help`, built from
`884d81b` (source: verb `mission` and verb `build` in `internal/verbs/registry.go`).

## Commands and flags (delivered)

Usage: `aayu mission <start|show|next|gate|budget|stop|resume|extend|tick|recover|retry|wake> [feature] [flags]`.

| Action | Who | What it does (from the verb help) |
|---|---|---|
| `start <f>` | person, device flow | Starts the approved mission; records the person, their tool for this repository, and the budget; then starts the Kanha on that tool. Given a Tatkal Feature's story, it starts that story on the person's own tool. Refused for an agent lane |
| `show <f>` | anyone | Prints the plan, each number with its source |
| `next <f>` | the Kanha | Lists what can start now, in order (priority, slice order, Ready time), and why each other one waits, with no model call; claims each start in `missions/<f>/claims.jsonl`; lists every event since `events_cursor` |
| `gate <f>` | the Kanha or a loop | Runs the gate of each milestone whose last story merged on the dev tip, marks it passed or failed, and lists one fix per failed check |
| `budget <f>` | person for `--to` | Raises the budget with a reason, or keeps a fix PR waiting (`--keep`) |
| `stop <f>` | person, device flow, any tool | Stops the mission, or one story or Bug with `--story`; `--end` ends it, only by its starter |
| `resume <f>` | person, on the mission's tool | Continues the mission or one story, up to its agents |
| `extend <f>` | person, device flow | Gives a mission at its time limit another `[mission] max_hours`, with `--reason` |
| `tick` | the build-ready loop | Hourly safety tick, with no model and no Feature |
| `recover <f>` | the Kanha, on the mission's tool | Reads the record and each host, re-attaches live threads, restarts lost ones under a claim |
| `retry <f> --story <n>` | person, on the mission's tool | One more attempt of a failed step |
| `wake <f>` | an event host or a timer | Gives a local Kanha its events every `[mission] check_minutes`, or starts the next stories on a cloud tool |

Flags: `--story`, `--reason`, `--end`, `--pr` (with gate: record a fix PR whose body has
`Aayudha-Gate-Fix: <milestone> <check>`), `--once-more`, `--to`, `--keep`. The last three are human-only
(`humanOnly` in the registry). Global flags include `--json`, `--plain`, and `--dry-run`.

`aayu build` flags this Feature uses: `--delegate <codex|cursor|claude|amp|opencode|auto>`, `--local`, `--thread`
(record the Amp thread a host created), `--stop <reason>`, and `--next ship|pick` (the build-ready loop).

## Terminal output (delivered)

`aayu mission next` prints one line per item, built by `emitWorkList` (`internal/cli/missionnext.go`):
`<action> #<n>[ (bug)][: <reason>][ (<detail>)][ · subthread <id>]`, then a note, then
`event #<n> at <time>: <event>` per event. Reasons in the code include `blocked`, `changed while locked`,
`claimed by another run`, and `no free agent` (`listNext`). Examples from the Completion Goal of
[#998](https://github.com/rahulnakmol/agentic-ayudha/issues/998): `start #20`, `waits #22: no free agent`,
`waits #24: amp paused until 20:10`.

When a mission is done, `next` prints: `The mission of #<f> is done: nothing starts. A new Bug of #<f> starts from its Release's Tatkal mission.` (`Env.workList` in `missionnext.go`).

`aayu mission gate` prints `Gate of #<f> on dev <sha>` and one line per milestone (`renderGate`,
`internal/cli/missiongate.go`). Gate fix lines read, for example, `Fix PR #<n> is the fix of <milestone> <check> (dev <sha>) in the mission of #<f>: build it.`

The Kanha's one-line report has this form (`skills/mission/SKILL.md`):
`merged #40: started #20 and #21 on amp; #22 waits: no free agent.`

The message ship sends the Kanha after a merge: `merged #<n> (PR #<pr>): run aayu mission next <f>`
(`internal/cli/ship.go`).

## Errors (delivered)

| Situation | Message | Source |
|---|---|---|
| An agent runs a human-only action | `<what> is a person's decision and is refused for an agent lane (<lane>); ask a person to run it in their own terminal` | `internal/cli/presence.go` |
| `next` or `recover` on another tool | `#<f>'s mission runs on <tool>, the tool of @<login>, who started it: run this there` | `missionnext.go`, `missionrecover.go` |
| A milestone gate failed `max_no_go` times | `<MS> of #<f> failed its gate <n> times: its stories wait` | `noGoTitle`, `missiongate.go` |
| A Kanha cannot start here | `a local Kanha on <tool> needs <Tool>'s CLI on this machine's PATH`, or `aayu starts no Kanha on <tool>` | `kanhaCanStart`, `internal/cli/kanha.go` |

Exit codes: 4 blocked (halted, paused, not approved, other tool), 5 GitHub or ledger failure, 6 budget (from
`skills/mission/SKILL.md`).

## Inbox cards (delivered)

| Card | Title and options | Source |
|---|---|---|
| Start | `Start the mission of #<f>`. Lines: how many issues become read-only until the last milestone, what is locked (each body, and its effort, risk, and priority labels), what stays open (comments and `status:` labels), when it ends, the tool, and `Agents  up to <n> stories at once`. Options: `1 Start the mission` (runs `aayu mission start <f>`), `2 Not now: nothing is locked`, `3 Read the issues first` | `startCard`, `lockItems`, `internal/cli/missionlock.go` |
| Locked edit | `#<n> changed while locked by the mission of #<f>`. Options: `1 Keep the edit: #<n> is planned again from its new body`, `2 Leave #<n> waiting`, `3 Open #<n> on GitHub to undo the edit yourself` | `lockItems` |
| Time | `#<f> has run <h> h`, with no recommendation | `missionCards`, `internal/cli/missiontime.go` |
| Gate no-go and budget asks | one card per ask on the Feature | `noGoAsk`, `budgetAsk`, `internal/cli/missionask.go` |

## Agent-facing surface (delivered)

- **Kanha brief** (`KanhaBrief`, `internal/delegate/brief.go`): four steps (recover, next, one build per `start`,
  report one line) and rules (start only what the list marks, never merge or ship, one tool only, stop on an
  error). On Amp, the brief explains the host hand-off when the amp CLI is not signed in.
- **Mission skill** (`skills/mission/SKILL.md`, "Kanha's protocol"): the same steps, exit codes, and the rule
  "Never restate text you did not act on" ([#1132](https://github.com/rahulnakmol/agentic-ayudha/issues/1132)).
- **Story brief** (`delegate.Brief`): the Completion Goal, how to work and report, the story text as data, merged
  work it builds on, the plan, the evidence map, branch and PR rules, checks, and the review loop.
- **MCP**: the verb is on CLI, skill, and MCP surfaces (`Surfaces: cliSkillMCP`); schema
  `internal/mcp/schemas/mission.json`.
- **Loop**: `loops/build-ready.toml` runs `aayu mission tick` hourly and then `aayu ship` for each PR it lists.

## Planned UX (not built)

| Story | Surface | Words fixed by the issue |
|---|---|---|
| [#1010](https://github.com/rahulnakmol/agentic-ayudha/issues/1010) | One progress comment on the Feature, edited in place | Starts `3 of 7 merged · 2 building · 1 blocked`, one row per story (C3) |
| [#1011](https://github.com/rahulnakmol/agentic-ayudha/issues/1011) | Pages to @rahulnakmol | `Milestone MS1 of #889 passed on dev <sha>: 3 merged. Startable now: #63, #64.`; `Mission #889 is done: 15 of 15 merged, 4 of 4 milestones passed` (C1, C3) |
| [#1014](https://github.com/rahulnakmol/agentic-ayudha/issues/1014) | One approval card for the proposed stories; `aayu mission feedback <f> --approve`, human-only; end page `feedback: none` | C5, C6, C8 |
| [#1030](https://github.com/rahulnakmol/agentic-ayudha/issues/1030) | Inbox card `PR #<pr> for #<n> waits for your review`; an undeclared high-risk card with `1 Review the PR` and `2 Stop #<n>`; next line `waits #<n>: undeclared high-risk change` | C2, C3, C5; "its words go into the UX baseline first" (Notes). #889 has no UX baseline on the ledger |
| [#1104](https://github.com/rahulnakmol/agentic-ayudha/issues/1104) | `## Build loop` section in the story brief; PR-body lines `Outcome (restated):` and `Customer value:` | C1, C3 |
| [#1109](https://github.com/rahulnakmol/agentic-ayudha/issues/1109) | `aayu build <n> --red`; warning `criterion 2 has no red run: …` | C1, C3 |
| [#1110](https://github.com/rahulnakmol/agentic-ayudha/issues/1110) | Journey files; refusal `journeys run only against the GitHub fake`; warning `#60 has no journey: build loop step 5 was not run` | C5, C6 |
| [#1111](https://github.com/rahulnakmol/agentic-ayudha/issues/1111) | `beyond-ask` finding: `no criterion of #60 asks for this` | C1 |
| [#1112](https://github.com/rahulnakmol/agentic-ayudha/issues/1112) | One inbox card to approve the next milestone's order | C2, C3 |
| [#1126](https://github.com/rahulnakmol/agentic-ayudha/issues/1126) | `aayu install --loops` report names the wake loop; `aayu mission show` says `#889 paused: its Kanha runs on rahulnakmol@laptop, which is off; it resumes at the next check` | C1, C8 |

## Accessibility

No source sets an accessibility requirement for these commands. Every output honours the global `--plain` flag
(words only: no colour, motion, emoji, or box drawing), and `--json` gives a machine-readable form (`aayu mission
--help`, Global Flags). The start card uses column words ("Locked", "Open", "Ends", "Agents") rather than symbols
(`startCard`). Screen-reader behaviour of the inbox cards is not checked here (unverified).
