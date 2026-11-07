# #1170 Specification

Sources: the spec baselines `specs/1170/baselines/spec-v1.json`, `spec-v2.json`, and `spec-v3.json` (all approved by
@rahulnakmol through the device flow; v3 at 2026-11-02T17:33:39Z), `specs/1170/approved.json` (approval version 3),
the Completion Goal of each issue, and `CONSTRAINTS.md` on `884d81b`. Spec v3 pins ten
Stories: #1172 to #1179, #1250, and #1254. The Bugs are not pinned in the spec; they are in the mission graph `missions/1170/plan.json`.
Story #1281 is in neither (see FR-11). "Status" in each heading is the state on `dev` `bc78349`, read on 2026-11-07
after the mission ended. "Closed unmerged" means the end of the mission closed its PR on 2026-11-07 and kept the
branch ([05-implementation-plan.md](05-implementation-plan.md), "Closed unmerged").

## Functional requirements

### FR-1 Baseline cards in the inbox (Story [#1172](https://github.com/rahulnakmol/agentic-ayudha/issues/1172), delivered)

1. Each waiting baseline draft (intent, spec, tech) is one Baseline card with the part, its version, its pins, and
   the counsel of the part's owner seat (criterion 1).
2. Approving three cards in one session asks for one device code and writes `specs/<f>/baselines/<part>-v<k>.json`
   as `aayu ready <f> --part <part>` writes them (criterion 2).
3. A draft that changed after the card was read writes nothing, and the card says
   `The spec of #889 changed after the draft. An agent drafts it again.` (criterion 3).
4. An agent lane sees the cards in `aayu inbox --list --json` with no approve action (criterion 4).
5. The spec card lists each story with title, criteria count, and effort, then a checklist (criterion 5).
6. A v2 draft shows each change since v1: what, why, and the need (criterion 6).

### FR-2 Approve to build card (Story [#1173](https://github.com/rahulnakmol/agentic-ayudha/issues/1173), delivered)

1. When the three baselines are approved and there is no approval, the card shows stories and Bugs, budget
   (stories × `[budget] per_story_usd`), agents, milestones, and high-risk entries (criterion 1).
2. Approving in the baselines' session needs no second code and writes `specs/<f>/approved.json` and
   `missions/<f>/plan.json` (criterion 2). Setting agents to 10 records `agents 10` (criterion 3).
3. A story that changed after the card was read approves nothing (`The stories of #889 changed while you read it.`,
   criterion 4). No tech baseline means no card (criterion 5).

### FR-3 Bugs come last (Story [#1174](https://github.com/rahulnakmol/agentic-ayudha/issues/1174), delivered)

1. Approve to build puts every open Bug in a final BUGS milestone, with or without `--milestones`; no open Bug means
   no BUGS milestone (criteria 1, 2, 5).
2. A Bug filed under the Feature during its mission joins BUGS, after the stories of earlier milestones (criterion 3).
3. The BUGS gate fails with `#31 merged with no proof pack` when a merged Bug has no pack (criterion 4).

### FR-4 The morning is set (Story [#1175](https://github.com/rahulnakmol/agentic-ayudha/issues/1175), delivered)

1. The inbox shows cards in this order: decisions, baselines, Approve to build, mission start, then the rest
   (criterion 1).
2. After the person approves and starts a Feature, the last card says `The day is set` with today's milestone, its
   stories, and the evening review time (criterion 2). Leaving before Mission start shows no such line (criterion 3).

### FR-5 The evening review (Story [#1176](https://github.com/rahulnakmol/agentic-ayudha/issues/1176), delivered)

1. At the end of `[time] business_hours`, one Evening card per story agreed that morning shows each criterion, its
   evidence kind, and its evidence from the pack (an inline image where the terminal shows images, else path and
   link) (criterion 1).
2. Accept writes a line to `missions/<f>/reviews.jsonl` (criterion 2). A comment is posted on the story for the
   mission's feedback pass (criterion 3).
3. A story that did not merge says `not merged:` with the reason and the next step (criterion 4).
4. An agent lane lists Evening cards with no accept action (criterion 5).

### FR-6 The day is proven (Story [#1177](https://github.com/rahulnakmol/agentic-ayudha/issues/1177), delivered)

1. Only when every agreed story merged, every criterion has evidence, and each is accepted, the inbox shows
   `The day is proven` with counts and the celebration, and records the milestone `day-proven` for the date
   (criterion 1).
2. A missing criterion shows no proven line and names it (`#1011 criterion 3 has no evidence`) (criterion 2).
3. `[ui] plain = true` keeps the words with no motion (criterion 3). No Feature started means no proven line
   (criterion 4).

### FR-7 The overnight card (Story [#1178](https://github.com/rahulnakmol/agentic-ayudha/issues/1178), delivered, PR #1238, `bc78349`)

1. After the evening review, an Overnight card proposes the next Feature's authoring, the verify sweep, and the evals,
   each with an hours and a cost bound (criterion 1).
2. Approving records `missions/overnight/<date>.json` with each run, its bounds, and its end at the next start of
   `[time] business_hours` (criterion 2).
3. After business hours, `aayu mission next` starts nothing and says `outside the day: starts at <time>`
   (criterion 3).
4. A run that reaches a bound stops, and the morning inbox names it (criterion 4). Leaving the card starts nothing
   except loops scheduled before (criterion 5).
5. Criterion 4 holds for the record only: "the tick stops a run and the morning names it. No agent carries out a run
   yet, so no process stops" (PR #1238 body). The accepted follow-ups F1, F2, and F4 hold the rest
   ([05-implementation-plan.md](05-implementation-plan.md), "Pending").

### FR-8 grill edits a filed body (Story [#1179](https://github.com/rahulnakmol/agentic-ayudha/issues/1179), delivered)

1. `aayu grill <type> --edit <n> --body <file>` checks the new body, writes it through the actions log, and prints
   the diff; a failing body writes nothing (criteria 1, 2).
2. A Ready issue is refused with `#1169 is Ready; its body is frozen`; a mission-locked issue is refused, naming the
   mission (criteria 3, 4). `--title` with `--edit` writes both in one write (criterion 5).
3. grill checks link targets as Ready does (`caused-by #999: #999 is a story; caused-by links to an incident`), and
   grill and `aayu slice` read comma-separated Expected paths the same way (criteria 6, 7).

### FR-9 Draft-first CI (Story [#1250](https://github.com/rahulnakmol/agentic-ayudha/issues/1250), closed unmerged, PR #1261)

1. A push to a draft PR runs no `ci.yml` job and no merge-gate or inspect job (criterion 1).
2. Marking it ready runs every required job and the merge gate once (criterion 2). Ready-PR pushes and pushes to
   `dev`, `rolling`, `main`, or a dispatch run as today (criteria 3, 4).
3. The amp brief says draft, `bash .agents/check --full`, a local review, then ready once (criterion 5).
4. `aayu ship` on a draft refuses with `#<pr> is a draft: mark it ready after the local gate passes` (criterion 6).

### FR-10 Two person windows a day (Story [#1254](https://github.com/rahulnakmol/agentic-ayudha/issues/1254), closed unmerged, draft PR #1270)

1. `[time] windows` defaults to 10:30-11:30 and 18:30-19:30 in `[time] tz`; this repository sets
   `tz = "Asia/Kolkata"` (criterion 1).
2. Past `[mission] max_hours` or the mission budget outside a window, `mission next` still lists startable stories,
   and the card waits for the next window; an unanswered card holds new starts from the end of that window
   (criteria 2, 3, 4). The `[budget]` breaker still stops every agent lane (criterion 4).
3. Outside a window nothing asks a person to act; at the window's start one message lists every queued item
   (criterion 5).
4. In a window the inbox holds every item that needs a person, in decision order, with one device code (criterion 6).
5. The day plan and evening review name the windows, not the end of business hours (criterion 7).

### FR-11 Every agent PR merges on proof (Story [#1281](https://github.com/rahulnakmol/agentic-ayudha/issues/1281), closed unmerged, draft PR #1282; not in the approved spec)

1. `[autonomy] always_person` lists no path that Decision [#1255](https://github.com/rahulnakmol/agentic-ayudha/issues/1255)
   frees; `always_person_config` keeps `.aayudha/config.toml` and `internal/config/defaults.toml` (criterion 1).
2. A proven PR that changes `internal/cli/ship.go` merges on proof at rung A or B (criterion 2). The policy files and
   a workflow that reads a secret or names an environment still wait for a person (criterion 3).
3. Each merge of a freed path gets an audit card in the next window, every time (criterion 4). Rung C merges only
   when `[autonomy] proof_rungs` lists C (criterion 5).

### FR-12 Bugs of the mission (BUGS milestone and MS1)

Each row is one Bug. "Criteria" summarises its Completion Goal; the full text is in the issue.

| FR | Bug | Requirement (criteria) | Status |
|---|---|---|---|
| 12.1 | [#1182](https://github.com/rahulnakmol/agentic-ayudha/issues/1182) | A person in an orb Terminal (`AMP_THREAD_ID` set, no `AAYU_AGENT`) gets the device code for Approve to build and mission start; `AAYU_AGENT=1` and `--ci` still refuse (1-6) | delivered |
| 12.2 | [#1191](https://github.com/rahulnakmol/agentic-ayudha/issues/1191) | A loop thread is recorded as a loop lane before it reads tasks, and its person-only commands are refused; a person's own tab still approves (1-4) | closed unmerged |
| 12.3 | [#1192](https://github.com/rahulnakmol/agentic-ayudha/issues/1192) | ship merges on the approver's `inspect --approve` of the exact head with no second code (1-5) | closed unmerged |
| 12.4 | [#1193](https://github.com/rahulnakmol/agentic-ayudha/issues/1193) | The user config sets the person's tool per repository; the repository config may not (1-3) | delivered |
| 12.5 | [#1201](https://github.com/rahulnakmol/agentic-ayudha/issues/1201) | A Decision superseded through an accepted successor's chain closes; a cycle closes nothing (1-4) | pending |
| 12.6 | [#1206](https://github.com/rahulnakmol/agentic-ayudha/issues/1206) | `ship --solo-bypass` asks for the code only after the read-only checks (1-4) | closed unmerged |
| 12.7 | [#1207](https://github.com/rahulnakmol/agentic-ayudha/issues/1207) | The mission keeps a story on an open `Refs` PR built outside it, adopts it, and syncs it (1-3) | delivered |
| 12.8 | [#1208](https://github.com/rahulnakmol/agentic-ayudha/issues/1208) | An Amp mission starts a story through a host hand-off when the amp CLI is not signed in (1-6) | delivered |
| 12.9 | [#1210](https://github.com/rahulnakmol/agentic-ayudha/issues/1210) | A directory in Expected paths covers its files; every `internal/` package falls in a layer (1-5) | delivered |
| 12.10 | [#1211](https://github.com/rahulnakmol/agentic-ayudha/issues/1211) | A stale amp environment record is caught by a dev check and re-verified by the cloud probe (1-5) | delivered |
| 12.11 | [#1226](https://github.com/rahulnakmol/agentic-ayudha/issues/1226) | The gate finds a pnpm-installed Playwright; a missing one is `unreachable`, with no fix listed (1-3) | delivered |
| 12.12 | [#1230](https://github.com/rahulnakmol/agentic-ayudha/issues/1230) | A gate fix PR records every failed check it names, and counts once against the budget (1-4) | delivered |
| 12.13 | [#1234](https://github.com/rahulnakmol/agentic-ayudha/issues/1234) | ship refuses a PR whose head does not contain the dev tip; the mission shows `waits on a rebase` (1-4) | closed unmerged |
| 12.14 | [#1248](https://github.com/rahulnakmol/agentic-ayudha/issues/1248) | A person may extend a mission in an extend window before its limit; an Extend card shows it (1-5) | closed unmerged |
| 12.15 | [#1251](https://github.com/rahulnakmol/agentic-ayudha/issues/1251) | `aayu ready` names only commands that exist (no `--ack`); a test checks every named flag (1-3) | closed unmerged |
| 12.16 | [#1252](https://github.com/rahulnakmol/agentic-ayudha/issues/1252) | `--dry-run` of Approve to build, `mission start`, `mission budget --to`, and `mission extend` sends no device-flow request (1-3) | closed unmerged |
| 12.17 | [#1253](https://github.com/rahulnakmol/agentic-ayudha/issues/1253) | One inbox card per Feature approves every part, then Approve to build, then mission start, after one code (1-5) | closed unmerged |
| 12.18 | [#1256](https://github.com/rahulnakmol/agentic-ayudha/issues/1256) | The build brief has a Goldfish review step by a model of another family; it is a preview, never a verdict (1-3) | pending |
| 12.19 | [#1267](https://github.com/rahulnakmol/agentic-ayudha/issues/1267) | Approve to build again keeps an unchanged milestone's passed gate (1-3) | closed unmerged |
| 12.20 | [#1268](https://github.com/rahulnakmol/agentic-ayudha/issues/1268) | Gate failures count toward `max_no_go` only since the milestone's last pass (1-3) | closed unmerged |
| 12.21 | [#1274](https://github.com/rahulnakmol/agentic-ayudha/issues/1274) | Every human-only verb has a dry run with no device flow, and a test walks every `presenceCheck` caller (1-3) | pending |
| 12.22 | [#1280](https://github.com/rahulnakmol/agentic-ayudha/issues/1280) | `mission resume` does not move the events cursor past unread events (1-3) | pending |
| 12.23 | [#1283](https://github.com/rahulnakmol/agentic-ayudha/issues/1283) | Retro eval cases use fixed dates, never the runner's clock (1-3) | pending |

## Non-functional requirements

| Area | Requirement | Source |
|---|---|---|
| Security: presence | Each person-only action (baseline, Approve to build, mission start, accept, overnight approval) needs a person's device code; one code covers one inbox session | #1172 criterion 2, #1173 criterion 2, #1178 criterion 2; `internal/cli/presence.go` `presenceCheck` (named in the #1170 body) |
| Security: agent lanes | An agent lane may list cards (`aayu inbox --list`) but never approve, accept, or extend | #1172 criterion 4, #1176 criterion 5, #1248 criterion 5, #1253 criterion 5; `aayu inbox --help` |
| Security: who approves | Changing who may approve is out of scope | #1170 Scope, Out |
| Reliability: drift | An approval re-reads its draft or stories after the code and writes nothing when they changed | #1172 criterion 3, #1173 criterion 4, #1253 criterion 3 |
| Reliability: gates | A milestone's gate is decided by fixed rules, not a model; after `mission.max_no_go` = 3 failed runs the mission stops and asks a person | `internal/cli/missiongate.go` (header comment); `CONSTRAINTS.md` `mission.max_no_go` |
| Reliability: merge order | A PR of a later milestone merges only after every earlier milestone passed its gate | `internal/cli/missionstop.go` `missionHold` |
| Cost: per story | `budget.per_story_usd` = 60.0; the approval budget is 10 stories × 60 = 600 USD | `CONSTRAINTS.md`; `specs/1170/approved.json` `budget` |
| Cost: mission | The mission budget was raised from 480 to 660 to 2500 USD by the person | `missions/1170/plan.json` `budget_basis` |
| Cost: overnight | Each overnight run has an hours bound and a cost bound | #1178 criteria 1, 4 |
| Cost: CI | Draft PRs run no paid CI or review | #1250 criteria 1, 2 |
| Time | A mission starts no new story after `mission.max_hours` = 48 working hours until a person extends it | `CONSTRAINTS.md` `mission.max_hours` |
| Quality floors | Changed-line coverage at least 90.0 % (`verify.floors.coverage.min_diff_percent`) | `CONSTRAINTS.md`; proof pack comments on PR #1238 (91.8) and PR #1261 (91.4) |
| Accessibility | `--plain` and `[ui] plain = true` give the same words with no colour, motion, emoji, or box drawing | #1177 criterion 3; `aayu inbox --help` (global flag `--plain`) |
| Operability: test world | Each Story's criteria run in `go test` of `internal/cli` with a fake GitHub, ledger, clock, and device flow, "never production" | Environment section of #1172 to #1179 |
| Operability: rollback | Each card kind and `--edit` go away with a revert; new ledger files stay readable and unused; no record is rewritten | `specs/1170/design.json` `rollback` |
| Performance | No source sets a latency or throughput target for this Feature | none found (the evidence does not show one) |
