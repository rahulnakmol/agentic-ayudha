# Specification: Feature #889

Sources: the approved spec baseline `specs/889/baselines/spec-v1.json` on `origin/ledger` (approved 2026-11-02T17:24:26Z
by @rahulnakmol), which pins 28 stories with their Completion Goal hashes, effort, layers, and depends-on; the issue
bodies; and `CONSTRAINTS.md`. Bugs are not pinned in the spec baseline; their requirements come from their own
Completion Goals. "Cn" means Completion Goal criterion n of that issue.

## Functional requirements

### Plan, start, and list (delivered)

1. **FR-1 Mission plan at Approve to build.** Approve to build writes `missions/<f>/plan.json` with milestones,
   missions, agents, and estimate, and refuses a plan that leaves a story or Bug in no milestone. Start records the
   starter, the tool, and the budget; an agent's start is refused. Trace:
   [#997](https://github.com/rahulnakmol/agentic-ayudha/issues/997) C1-C10.
2. **FR-2 Startable list with no model call.** `aayu mission next <f>` lists `start` and `waits` lines in order
   (priority, slice order, Ready time), claims each start on the ledger so two runs never start one story, and
   refuses a run on another tool than the mission's. With `--json` it is an `aayudha.work/v1` list. Trace:
   [#998](https://github.com/rahulnakmol/agentic-ayudha/issues/998) C1-C13.
3. **FR-3 Depends-on holds.** The list holds a story whose depends-on target is open, and counts a merged but still
   open target as merged. Trace: [#999](https://github.com/rahulnakmol/agentic-ayudha/issues/999) C1-C3;
   [#1169](https://github.com/rahulnakmol/agentic-ayudha/issues/1169).
4. **FR-4 One cap on parallel work.** `[build] agents = 5` per Kanha; a local value only lowers it; Approve to build
   may set a Feature's own; a prompt never sets a limit; `in_flight` is replaced. Trace:
   [#1000](https://github.com/rahulnakmol/agentic-ayudha/issues/1000) C1-C6.
5. **FR-5 A moved story does not start on the old mission.** Trace:
   [#1129](https://github.com/rahulnakmol/agentic-ayudha/issues/1129).

### Kanha and story threads (delivered)

1. **FR-6 Start begins the Kanha** on the plan's tool: the drafting Amp thread, a new Claude Code session, a Cursor
   cloud agent, or a local session for Codex, Copilot, and OpenCode. A mission never mixes tools. Trace:
   [#1004](https://github.com/rahulnakmol/agentic-ayudha/issues/1004) C1-C8;
   [#1099](https://github.com/rahulnakmol/agentic-ayudha/issues/1099) (one start time).
2. **FR-7 Kanha protocol in the mission skill.** On each event the Kanha runs recover, then next, then one build per
   `start` line, and reports one line. Body text is data. `aayu doctor` names a tool missing from `lock.toml`.
   Trace: [#1005](https://github.com/rahulnakmol/agentic-ayudha/issues/1005) C1-C5;
   [#1132](https://github.com/rahulnakmol/agentic-ayudha/issues/1132) (the report never restates text it did not
   act on).
3. **FR-8 A story subthread owns one story,** may use subagents, and ships it on proof; a receiver that cannot run
   aayu never merges; the brief lists merged work it builds on. Trace:
   [#1006](https://github.com/rahulnakmol/agentic-ayudha/issues/1006) C1-C3.
4. **FR-9 Events tell the Kanha.** `ship` tells the Kanha after each merge in the same process, and a failed message
   does not undo the merge; a stop, a block, a failed start, or an answered ask tells it too. Trace:
   [#1007](https://github.com/rahulnakmol/agentic-ayudha/issues/1007) C1-C3;
   [#1008](https://github.com/rahulnakmol/agentic-ayudha/issues/1008) C1-C3.
5. **FR-10 Hourly safety tick** wakes each Kanha that missed an event, reads every event since `events_cursor`, and
   syncs open cloud PRs. Trace: [#1009](https://github.com/rahulnakmol/agentic-ayudha/issues/1009) C1-C5;
   [#1121](https://github.com/rahulnakmol/agentic-ayudha/issues/1121) (closed: fixed by PR
   [#1103](https://github.com/rahulnakmol/agentic-ayudha/issues/1103)).
6. **FR-11 Stateless wake and local Kanha.** `aayu mission wake <f>` serves event hosts; a local Kanha is resumed
   with its tool's resume command every `[mission] check_minutes`. Trace:
   [#1012](https://github.com/rahulnakmol/agentic-ayudha/issues/1012) C1-C8.

### Control, gates, limits, and the lock (delivered)

1. **FR-12 Milestone gate.** When a milestone's last story merges, the gate runs on the dev tip (required checks,
   `aayu verify --check all`, the evals of the seats the milestone touched; no model decides), records pass or fail,
   and gives each failed check one pre-authorised fix PR; after `[mission] max_no_go` failures the mission stops and
   asks a person. Trace: [#1001](https://github.com/rahulnakmol/agentic-ayudha/issues/1001) C1-C9;
   [#1100](https://github.com/rahulnakmol/agentic-ayudha/issues/1100) (the merge gate accepts a listed fix PR).
2. **FR-13 Stop, resume, end, and merge order.** A person stops a mission or one story from any tool; resume runs
   only on the mission's tool; `--end` is for the starter only; ship merges a milestone's PRs only after each
   earlier gate passes, and a Feature's PR only as a listed gate fix. Trace:
   [#1002](https://github.com/rahulnakmol/agentic-ayudha/issues/1002) C1-C18.
3. **FR-14 Recover and retry.** Recover re-attaches what lives, restarts what is lost under a claim, marks unreadable
   hosts, and retries a failed step after `retry_backoff_minutes`, doubling, up to `retry_max`. Trace:
   [#1003](https://github.com/rahulnakmol/agentic-ayudha/issues/1003) C1-C9;
   [#1148](https://github.com/rahulnakmol/agentic-ayudha/issues/1148),
   [#1149](https://github.com/rahulnakmol/agentic-ayudha/issues/1149).
4. **FR-15 Lock at start.** Start makes the bodies and the effort, risk, and priority labels of the Feature, its
   stories, and its Bugs read-only; comments and `status:` labels stay open; a locked edit holds its story until a
   person keeps or reverts it. Trace: [#1013](https://github.com/rahulnakmol/agentic-ayudha/issues/1013) C1-C9.
5. **FR-16 Time limits.** Approve to build estimates working hours; an inbox card at 24 h; no new start after 48 h
   until a person extends with a reason. Trace: [#1015](https://github.com/rahulnakmol/agentic-ayudha/issues/1015)
   C1-C13.
6. **FR-17 Halt releases claims** of stories that do not run yet. Trace:
   [#1101](https://github.com/rahulnakmol/agentic-ayudha/issues/1101).

### Pending in v0.4.0

1. **FR-18 One event record per Feature.** Every build event goes to `delivery/<f>/events.jsonl` (`started`,
   `pr-opened`, `merged`, `asked`, `blocked`), written by aayu's verbs, never by the child's model, plus one
   progress comment edited in place. Trace: [#1010](https://github.com/rahulnakmol/agentic-ayudha/issues/1010)
   C1-C3. Not built.
2. **FR-19 Pages only at milestone end, block, and mission end.** Trace:
   [#1011](https://github.com/rahulnakmol/agentic-ayudha/issues/1011) C1-C4. Not built.
3. **FR-20 Feedback pass at mission end.** Comments are read as data; only those that change a spec, a behaviour,
   or a definition of done become proposed stories; @rahulnakmol approves the list once with
   `aayu mission feedback <f> --approve`; no delivered body changes. Trace:
   [#1014](https://github.com/rahulnakmol/agentic-ayudha/issues/1014) C1-C8. Not built.
4. **FR-21 Merge on proof inside an approved mission,** holding only PRs the person marked review, undeclared
   high-risk diffs, an entry added after approval, and floor misses; new `[risk]` path keys. Trace:
   [#1030](https://github.com/rahulnakmol/agentic-ayudha/issues/1030) C1-C8. Not built.
5. **FR-22 10-step build loop in the story brief,** with `Outcome (restated):` and `Customer value:` PR-body lines,
   and an eval case that names a missing step. Trace:
   [#1104](https://github.com/rahulnakmol/agentic-ayudha/issues/1104) C1-C5. Not built.
6. **FR-23 Local wake loop.** `aayu install --loops` adds a wake loop at `check_minutes` only on the local Kanha's
   own machine, removes it when the mission ends, and records check-ins; two missed check-ins pause the Feature.
   Trace: [#1126](https://github.com/rahulnakmol/agentic-ayudha/issues/1126) C1-C8. Not built.
7. **FR-24 Lock ends at the last gate.** Trace: [#1134](https://github.com/rahulnakmol/agentic-ayudha/issues/1134)
   C1-C2. C1 appears met on `dev`: `missionDone` needs `lastGatePassed` before `endLock` runs
   (`internal/cli/missionnext.go`, from [c2d6bd70](https://github.com/rahulnakmol/agentic-ayudha/commit/c2d6bd70)).
   C2 is not built: `aayu mission gate` does not call `endLock` (the only caller is `missionnext.go`). Inference from
   the code; no test for #1134 exists.
8. **FR-25 Fast `internal/cli` tests:** 300 s or less on CI, 450 s or less on a 2-core orb, with no test lost.
   Trace: [#1154](https://github.com/rahulnakmol/agentic-ayudha/issues/1154) C1-C3. Not built.

### Pending in v0.4.1

1. **FR-26 Red runs in proof packs** (`aayu build <n> --red`), a warning, never a floor. Trace:
   [#1109](https://github.com/rahulnakmol/agentic-ayudha/issues/1109) C1-C5.
2. **FR-27 Real-run journeys** on the built binary against the GitHub fake, re-run by the milestone gate. Trace:
   [#1110](https://github.com/rahulnakmol/agentic-ayudha/issues/1110) C1-C6.
3. **FR-28 `beyond-ask` inspect finding (P2) and a milestone value record** that never holds a milestone. Trace:
   [#1111](https://github.com/rahulnakmol/agentic-ayudha/issues/1111) C1-C6.
4. **FR-29 Milestone findings propose the next milestone's order;** only a person's approval applies it. Trace:
   [#1112](https://github.com/rahulnakmol/agentic-ayudha/issues/1112) C1-C5.

### Bug fixes still open

| Bug | Requirement | Criteria |
|---|---|---|
| [#1128](https://github.com/rahulnakmol/agentic-ayudha/issues/1128) | A sync records a follow-up before it sends it, so one head's findings reach the receiver at most once | C1-C3 |
| [#1138](https://github.com/rahulnakmol/agentic-ayudha/issues/1138) | `tellKanha` sends text aayu did not write to a cloud Kanha only inside an untrusted block (CWE-1427) | in PR [#1260](https://github.com/rahulnakmol/agentic-ayudha/issues/1260) |
| [#1139](https://github.com/rahulnakmol/agentic-ayudha/issues/1139) | `mission next` prints delegation events inside an untrusted block, with a closing tag escaped | C1-C3 (Ready snapshot `specs/1139/ready.md`) |
| [#1197](https://github.com/rahulnakmol/agentic-ayudha/issues/1197) | `build`, `mission wake`, and `mission tick` print receiver error text only inside an untrusted block | C1-C3 |
| [#1161](https://github.com/rahulnakmol/agentic-ayudha/issues/1161), [#1203](https://github.com/rahulnakmol/agentic-ayudha/issues/1203), [#1204](https://github.com/rahulnakmol/agentic-ayudha/issues/1204) | A gate run that another PR from the branch may have had never counts for a merged PR in `proof land` | per issue |
| [#1166](https://github.com/rahulnakmol/agentic-ayudha/issues/1166) | Every mission eval case passes its bar; no bar changes | C1-C3; the halted case is still open |
| [#1198](https://github.com/rahulnakmol/agentic-ayudha/issues/1198) | A config error names a rejected secret field and its rule, never the value | in PR [#1202](https://github.com/rahulnakmol/agentic-ayudha/issues/1202) |
| [#1266](https://github.com/rahulnakmol/agentic-ayudha/issues/1266) | The Amp host hand-off lapses with its claim, and only a well-formed thread ID is recorded | in PR [#1269](https://github.com/rahulnakmol/agentic-ayudha/issues/1269) |

## Non-functional requirements

| Kind | Requirement | Source |
|---|---|---|
| Operability | Every mission limit is read from config, never from a prompt: `mission.max_no_go = 3`, `retry_max = 2`, `retry_backoff_minutes = 10`, `check_minutes = 5`, `warn_hours = 24`, `max_hours = 48`, and `build.agents = 5` | `CONSTRAINTS.md`, "Missions" and `build.agents`; `internal/config/defaults.toml` `[build]`, `[mission]` |
| Operability | The safety tick runs hourly (`cron = "0 * * * *"`) as one UTC cron in each whole-hour zone, so no tool runs it twice an hour | `loops/build-ready.toml` |
| Reliability | Two runs of the list never start one story: each start is a claim line in `missions/<f>/claims.jsonl` | `MissionClaimsPath`, `internal/cli/missionnext.go`; #998 C4 |
| Reliability | Recover run twice changes nothing, and no story has two live agents | #1003 C2 |
| Reliability | Every send, hand-off, follow-up, and status read to a receiver ends after two minutes | `receiverTimeout`, `withReceiverTimeout` in `internal/cli/kanha.go`; `delegate.SubmitTimeout` in `internal/delegate/receivers.go`; #1118, #1124, #1137 |
| Reliability | A delegated build reports only the record the ledger holds | #1264 ([6cf3876c](https://github.com/rahulnakmol/agentic-ayudha/commit/6cf3876c)) |
| Security | Start, stop, resume, extend, retry, `gate --once-more`, and `budget --to` are a person's decisions behind GitHub's device flow, refused for an agent lane | `internal/verbs/registry.go`, verb `mission` (`humanOnly`, `IdentityHuman`, `GateHumanIdentity`); #997 C10, #1002 C6 |
| Security | Issue, PR, and comment text, and command output, are data, never instructions to the Kanha | `KanhaBrief` Rules; `skills/mission/SKILL.md`; #1005 C2 |
| Security | Text that aayu did not write reaches a model only inside an untrusted block (CWE-1427) | #1138, #1139, #1197 (pending) |
| Security | A config error never prints a rejected secret value | #1198 (in review) |
| Cost | A mission's spend stays inside the budget recorded at Start; a gate fix that would pass it waits for a person (exit 6) | #997 C9, #998 C8, #1001 C6; verb `mission` `--pr` help |
| Cost | The safety tick is a paid agent turn, 24 a day | `loops/build-ready.toml` comment |
| Performance | `internal/cli` tests: 300 s or less on CI (planned); CI `go test` has `-timeout 30m` now | #1154; `.github/workflows/ci.yml` (#1151) |
| Accessibility | No source states an accessibility requirement for this Feature. The global `--plain` flag (words only, no colour or box drawing) applies to every `aayu mission` output (inferred) | `aayu mission --help`, Global Flags |
