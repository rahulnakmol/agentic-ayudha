# Technical design: Feature #889

Sources: the approved tech baseline `specs/889/baselines/tech-v1.json` (approved 2026-11-02T17:25:45Z), which pins
`specs/889/design.json`; the code on `dev` at `884d81b` (every cited symbol was checked again at `bc78349`, which
also changed `missionnext.go`, `missionrecover.go`, and `missiontick.go` for Story #1178); and the issue bodies. Every file path is relative to the
repository root. Parts marked **planned, not built** do not exist on `dev`.

## Architecture

```text
person ──aayu mission start <f>──▶ missions/<f>/plan.json  (+ lock, budget, tool)
   │                                specs/<f>/approved.json parent = the Kanha
   ▼
Kanha session (Amp thread, Claude Code, Cursor cloud agent, or a local Codex/Copilot/OpenCode/Cursor session)
   │  on each event: aayu mission recover <f> → aayu mission next <f> → aayu build <n> --delegate <tool> per "start"
   ▼
story subthread (one per story) ── PR ──▶ merge gate ──▶ aayu ship ──merge──▶ dev
   ▲                                                         │
   └──────────── tellKanha("merged #n …") ◀──────────────────┘
events also from: aayu build --stop / failed start, answered ask, aayu mission stop,
                  aayu mission tick (hourly, loops/build-ready.toml), aayu mission wake (event hosts, local timer)
milestone end ──aayu mission gate <f>──▶ missions/<f>/gates.jsonl, plan.milestones[].gate, gate_fixes
```

aayu decides what starts; the Kanha only acts on the list (`KanhaBrief`, `internal/delegate/brief.go`). No model
decides a gate (`aayu mission` help, `gate` action, `internal/verbs/registry.go`).

## Layers touched

`specs/889/design.json` gives the footprint L02, L03, L04, L05, L07. Paths are from `LAYERS.md`.

| Layer | Name | Paths this Feature changed or plans to change |
|---|---|---|
| L02 | Interaction / API boundary | `internal/cli/mission*.go`, `kanha*.go`, `delegate.go`, `build.go`, `buildnext.go`, `ship.go`, `shipproof.go`, `home.go`, `halt.go`, `inspect*.go`, `readyapprove.go` |
| L03 | Application / workflow | `internal/delegate/**`, `internal/inspect/**`, `internal/ready/**`, `internal/verbs/registry.go`, `internal/doctor/**`, `loops/**`, `skills/mission/SKILL.md` |
| L04 | Domain / policy | `internal/verify/**`, `internal/proof/**`, `CONSTRAINTS.md` (generated) |
| L05 | Intelligence | `evals/cases/mission/**`, `evals/eval.toml` |
| L07 | Operational data / state | `internal/config/**` (`defaults.toml`, `config.go`, `load.go`, `schema.json`), `internal/ledger/**` |
| L06 | Contracts / integration (not in the footprint, but changed) | `internal/github/fake/fake.go` (many PRs, for example [3ece84e7](https://github.com/rahulnakmol/agentic-ayudha/commit/3ece84e7)) |
| L09 | Platform / operations (not in the footprint, but changed) | `.github/workflows/ci.yml` ([3a59e635](https://github.com/rahulnakmol/agentic-ayudha/commit/3a59e635)), `.github/workflows/aayudha-merge-gate.yml` ([d40b7c9b](https://github.com/rahulnakmol/agentic-ayudha/commit/d40b7c9b)) |

## Packages and files (delivered)

| Part | File and symbol | Story or Bug |
|---|---|---|
| Plan at Approve to build | `internal/cli/mission.go`: `MissionPlan`, `approveRun.missionPlan`, `splitMissions`, `parseMilestones`, `checkMilestones` | [#997](https://github.com/rahulnakmol/agentic-ayudha/issues/997) |
| Start | `internal/cli/mission.go`: `missionStart`, `storyStart` (Tatkal), `repoTool`, `callerTool` | #997, [#1004](https://github.com/rahulnakmol/agentic-ayudha/issues/1004) |
| Kanha start and messages | `internal/cli/kanha.go`: `kanhaCanStart`, `localKanha`, `startKanha`, `tellKanha`, `answeredAsk`, `writeParent`, `withReceiverTimeout`; `internal/delegate/brief.go`: `KanhaBrief` | #1004, [#1007](https://github.com/rahulnakmol/agentic-ayudha/issues/1007), [#1008](https://github.com/rahulnakmol/agentic-ayudha/issues/1008), [#1118](https://github.com/rahulnakmol/agentic-ayudha/issues/1118) |
| Local Kanha | `internal/cli/kanhalocal.go`: `startLocalKanha`, `checkLocalKanha`, `kanhaPrompt` (wraps events in `<untrusted source=…>`), `kanhaCheckPath`; `internal/cli/kanhalock_unix.go`, `kanhalock_windows.go`; `internal/delegate/local.go`: `LocalKanha = {Codex, Copilot, OpenCode}` | [#1012](https://github.com/rahulnakmol/agentic-ayudha/issues/1012) |
| Startable list | `internal/cli/missionnext.go`: `missionNext`, `listNext`, `heldDeps`, `movedOut`, `emitWorkList`, `MissionClaimsPath`, `missionDone`, `lastGatePassed` | [#998](https://github.com/rahulnakmol/agentic-ayudha/issues/998), [#999](https://github.com/rahulnakmol/agentic-ayudha/issues/999), [#1129](https://github.com/rahulnakmol/agentic-ayudha/issues/1129), [#1169](https://github.com/rahulnakmol/agentic-ayudha/issues/1169) |
| Agents cap | `internal/cli/mission.go`: `Env.missionAgents`; `internal/config/defaults.toml` `[build] agents = 5` | [#1000](https://github.com/rahulnakmol/agentic-ayudha/issues/1000) |
| Milestone gate | `internal/cli/missiongate.go`: `missionGate`, `runGate`, `gateRequired`, `gateVerify`, `gateEvals`, `missionGateFix`, `judgeGateFix`, `MissionGatesPath`; `internal/cli/missionask.go`: `noGoAsk`, `budgetAsk`; `internal/cli/verify.go`: `GateFix` field of the verify result | [#1001](https://github.com/rahulnakmol/agentic-ayudha/issues/1001), [#1100](https://github.com/rahulnakmol/agentic-ayudha/issues/1100) |
| Stop, resume, end | `internal/cli/missionstop.go`: `missionStop`, `missionResume`, `endMission`, `missionHold` (called by `ship.go` before the land and again at the merge) | [#1002](https://github.com/rahulnakmol/agentic-ayudha/issues/1002) |
| Recover and retry | `internal/cli/missionrecover.go`: `missionRecover`, `missionRetry`, `recovery.reattach`, `recovery.restart`, `recovery.schedule` | [#1003](https://github.com/rahulnakmol/agentic-ayudha/issues/1003), [#1148](https://github.com/rahulnakmol/agentic-ayudha/issues/1148), [#1149](https://github.com/rahulnakmol/agentic-ayudha/issues/1149) |
| Safety tick | `internal/cli/missiontick.go`: `missionTick`, `tickMission`, `tickSync`, `missionEvents`; `loops/build-ready.toml` (`command = ["mission", "tick"]`) | [#1009](https://github.com/rahulnakmol/agentic-ayudha/issues/1009) |
| Stateless wake | `internal/cli/missionwake.go`: `missionWake`, `MissionHostWake` | #1012 |
| Lock | `internal/cli/missionlock.go`: `newLock`, `lockGuard.RoundTrip` (refuses a write before it leaves), `lockedEdits`, `endLock`, `keepEdit`, `startCard` | [#1013](https://github.com/rahulnakmol/agentic-ayudha/issues/1013) |
| Time limits | `internal/cli/missiontime.go`: `estimateMission`, `missionClocks`, `missionCards`, `missionExtend` | [#1015](https://github.com/rahulnakmol/agentic-ayudha/issues/1015) |
| Halt | `internal/cli/halt.go`: `releaseMissionClaims` | [#1101](https://github.com/rahulnakmol/agentic-ayudha/issues/1101) |
| Story subthread | `internal/delegate/brief.go`: `Brief` (sections "How to work and report", "Merged work this story builds on", "Branch, commits, and pull request"); `internal/cli/delegate.go`: `delegateStep`; `internal/delegate/receivers.go`: `AmpCLI` | [#1006](https://github.com/rahulnakmol/agentic-ayudha/issues/1006), [#1264](https://github.com/rahulnakmol/agentic-ayudha/issues/1264) |
| Mission skill | `skills/mission/SKILL.md`, "Kanha's protocol"; `internal/doctor/checks.go` (a tool not in `lock.toml`); `evals/cases/mission/{halted,starts-each,stays-in-list}` | [#1005](https://github.com/rahulnakmol/agentic-ayudha/issues/1005), [#1132](https://github.com/rahulnakmol/agentic-ayudha/issues/1132) |
| Merge gate and proof fixes found on the way | `internal/inspect/apps.go` (CodeRabbit icon badges, `/aayu` commands), `internal/cli/inspect.go` (approval lift, dismissal), `internal/cli/inspect_app.go` (idle reopen reply), `internal/cli/home.go`: `gateSuites`, `mergedRunOf`; `internal/cli/proof.go`, `ship.go` | [#1130](https://github.com/rahulnakmol/agentic-ayudha/issues/1130), [#1133](https://github.com/rahulnakmol/agentic-ayudha/issues/1133), [#1150](https://github.com/rahulnakmol/agentic-ayudha/issues/1150), [#1155](https://github.com/rahulnakmol/agentic-ayudha/issues/1155), [#1159](https://github.com/rahulnakmol/agentic-ayudha/issues/1159), [#1162](https://github.com/rahulnakmol/agentic-ayudha/issues/1162), [#1163](https://github.com/rahulnakmol/agentic-ayudha/issues/1163), [#1165](https://github.com/rahulnakmol/agentic-ayudha/issues/1165), [#1031](https://github.com/rahulnakmol/agentic-ayudha/issues/1031) |

## Data and ledger records

| Record | Where | Written by | Contents |
|---|---|---|---|
| Mission plan | `missions/<f>/plan.json` (`MissionPlanPath`) | Approve to build, Start, next, gate, stop, recover, tick, wake | `MissionPlan`: graph, milestones with `gate {state, dev_sha, at}`, missions, agents, estimate, `started_by`, `tool`, `budget`, `pause`, `state`, `stopped`, `lock`, `controls`, `gate_fixes`, `events_cursor`, `wakes`, `story_states`, `sources`, `host_wakes` (`internal/cli/mission.go`) |
| Claims | `missions/<f>/claims.jsonl` (`MissionClaimsPath`) | `aayu mission next`, recover, and a gate fix's start (`claimSlot`); release lines from build (`releaseStart`), the mission's end (`endMission`), and halt (`releaseMissionClaims`) | one line per start, so no story starts twice |
| Gate runs | `missions/<f>/gates.jsonl` (`MissionGatesPath`) | `aayu mission gate` | `GateRun` with `GateCheck` results |
| Kanha | `specs/<f>/approved.json`, field `parent` | Start (`writeParent`) | `ready.Parent`: host, session, URL, mode (cloud or local), machine, `last_check` (`internal/ready/approve.go`) |
| Delegation | `delegate/<n>/state.json` | `aayu build <n> --delegate` | `delegate.State`, including `feature` and the Kanha's session (#1004 C5) |
| Local check | `<state dir>/state/kanha/<f>.json`, not on the ledger | the local Kanha's check | last check time and events sent (`kanhaCheckPath`, `localCheck`) |
| Events record | `delivery/<f>/events.jsonl` | **planned, not built** ([#1010](https://github.com/rahulnakmol/agentic-ayudha/issues/1010)); `internal/delivery/` does not exist on `dev` | append-only `started`, `pr-opened`, `merged`, `asked`, `blocked` lines |
| Milestone learning | `missions/<f>/learning/<MS>.json` | **planned, not built** ([#1112](https://github.com/rahulnakmol/agentic-ayudha/issues/1112)) | findings with sources |
| Value record | `milestones[<MS>].value` in the plan | **planned, not built** ([#1111](https://github.com/rahulnakmol/agentic-ayudha/issues/1111)) | question, Outcome line, `Customer value:` lines, answer |
| Red runs | `red` list in `pack.json` | **planned, not built** ([#1109](https://github.com/rahulnakmol/agentic-ayudha/issues/1109)) | `{test, sha, failure, reason}` |

Rollback (`specs/889/design.json`, `rollback`): "Ledger files are append-only or versioned; a revert stops new writes
and leaves records readable."

## Config keys

| Key | Default | Source |
|---|---|---|
| `build.agents` | 5 | `internal/config/defaults.toml` `[build]`; `CONSTRAINTS.md` |
| `mission.max_no_go` | 3 | `[mission]` in `defaults.toml` |
| `mission.retry_max` | 2 | same |
| `mission.retry_backoff_minutes` | 10 | same |
| `mission.check_minutes` | 5 | same |
| `mission.warn_hours` | 24 | same |
| `mission.max_hours` | 48 | same |
| `risk.migration_paths`, `risk.infrastructure_paths`, `risk.authority_paths` | `["**/migrations/**", "**/*.sql"]`, `["**/*.tf", "**/*.bicep"]`, the narrow-set entries | **planned, not built** ([#1030](https://github.com/rahulnakmol/agentic-ayudha/issues/1030) C8); no `[risk]` table in `defaults.toml` |

`build` and `mission` are both in `autonomy.always_person_sections` (`CONSTRAINTS.md`), so a change to these tables
waits for a person.

## Contracts

- `aayu mission next --json` is an `aayudha.work/v1` list: `{schema, scope {kind, id}, items [{key, action, reason,
  subthread}], events [{at, story, event}]}` (`WorkList` in `internal/cli/missionnext.go`; verb help).
- MCP schema: `internal/mcp/schemas/mission.json`, generated from `internal/verbs/registry.go`.
- Exit codes the Kanha reads: 4 (halted, paused, not approved, not started, or another tool), 5 (GitHub or the
  ledger failed), 6 (the budget breaker paused agent lanes) (`skills/mission/SKILL.md`, "Kanha's protocol").
- A gate fix PR names its check on a line `Aayudha-Gate-Fix: <milestone> <check>` (verb help, `--pr`).
- Planned contracts in `specs/889/design.json`: the events record and progress comment (#1010), paging from it
  (#1011), ship inside an approved mission (#1030, Decision #1027 option 2), the 10-step brief (#1104), the
  feedback pass (#1014), the install loop (#1126), the lock ending at the last gate (#1134), and the v0.4.1 stories
  #1109 to #1112.

## Key decisions

| Decision | What it settles here |
|---|---|
| [#915](https://github.com/rahulnakmol/agentic-ayudha/issues/915) (done) | A mission runs in milestones with gates and merges in milestone order; gate-fix Bugs need no separate triage |
| [#974](https://github.com/rahulnakmol/agentic-ayudha/issues/974) (done) | A mission locks its issues at start, takes feedback as comments after it ends, gets gate fixes as PRs, and runs at most 48 hours |
| [#981](https://github.com/rahulnakmol/agentic-ayudha/issues/981) (done) | Every Feature names its customer outcome; "unknown" is valid and never blocks (#1111) |
| [#1027](https://github.com/rahulnakmol/agentic-ayudha/issues/1027) (done) | Inside an approved mission the approval replaces the per-PR person, and `risk:` means rollback impact (#1030 builds option 2) |
| [#1255](https://github.com/rahulnakmol/agentic-ayudha/issues/1255) (done, 2026-11-02) | Every agent PR into dev merges on proof at any risk and any hour. Story [#1281](https://github.com/rahulnakmol/agentic-ayudha/issues/1281) (Feature #1170) was to build it in draft PR [#1282](https://github.com/rahulnakmol/agentic-ayudha/issues/1282), closed unmerged on 2026-11-07 when the mission of #1170 ended (branch `feat/1281-auto-merge-on-proof` at `6f2aae7` kept; `_data/closed-prs.tsv`), so it is not built. This overlaps #1030; see [06-sequencing.md](06-sequencing.md) |

## Pending design, from the issue bodies (planned, not built)

- **#1010 events record.** A writer in `internal/delivery/`, which Feature
  [#872](https://github.com/rahulnakmol/agentic-ayudha/issues/872) creates; "this story adds the events writer there
  and creates no new package". The `asked` event needs `aayu ask` from Feature
  [#880](https://github.com/rahulnakmol/agentic-ayudha/issues/880); no `ask` verb is registered on `dev`
  (`rg 'register\("ask"' internal/cli`). The progress comment risks GitHub's rate limit (#1010, Review guidance).
- **#1011 paging.** aayu decides each page from `delivery/<f>/events.jsonl`; tested with a stub notifier in
  `internal/delegate` and `internal/cli`.
- **#1014 feedback pass.** Reads comments with the GitHub client's comment list; files stories through
  `aayu triage --decide story --changes` ([#879](https://github.com/rahulnakmol/agentic-ayudha/issues/879) S8, v0.4.1),
  which `dev` does not have; a new human-only `aayu mission feedback <f> --approve`; a skill protocol change.
- **#1030 ship inside a mission.** Reads the approval from the base branch's ledger and the PR's story (`StoryOf`,
  `internal/cli/verify.go`); matches high-risk patterns on the base and head as the narrow set does
  (`changedSections`, `internal/cli/shipproof.go`). Depends on Stories
  [#1028](https://github.com/rahulnakmol/agentic-ayudha/issues/1028),
  [#1029](https://github.com/rahulnakmol/agentic-ayudha/issues/1029), and
  [#1083](https://github.com/rahulnakmol/agentic-ayudha/issues/1083), all OPEN and `status:planned`.
- **#1104 build loop.** A `## Build loop` section in `delegate.Brief` and a tier-1 eval case `build/ten-step-loop`.
  Open question in the issue: `evals/` has no case kind for a rendered delegate brief.
- **#1126 wake loop.** `internal/cli/installloops.go` adds a local loop on the Kanha's machine; which scheduler each
  tool offers is "for the plan to check".
- **#1134 lock end.** `aayu mission gate` ends the lock in the ledger commit that records the last gate.
- **#1154 test speed.** Profile first (`go test -json`, sorted by Elapsed), then split, parallelise, or share fixtures.
- **#1128 send once.** Record the follow-up as `sending` before the send. The issue cites `delegate.go:807` and
  `:815`; those lines have moved. On `dev` the send is `delegation.followUp` (`internal/cli/delegate.go`), which calls
  `adapter.FollowUp` under `withReceiverTimeout`; `Env.reviewLoop` calls it.
- **#1139, #1197 untrusted blocks.** Reuse the block that `kanhaPrompt` (`internal/cli/kanhalocal.go`) makes with
  `triage.Untrusted` (`internal/triage/triage.go`). Open PR
  [#1260](https://github.com/rahulnakmol/agentic-ayudha/issues/1260) escapes every `<` in it and uses it for a cloud
  Kanha's follow-up (`tellKanhaData`). Bug #1197 names a helper `asUntrusted`; that name is not on `dev` and not in
  the diff of PR #1260.
