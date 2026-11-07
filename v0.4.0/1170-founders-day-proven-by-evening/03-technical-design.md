# #1170 Technical design

Sources: the tech baseline `specs/1170/baselines/tech-v1.json` (approved 2026-10-31T09:46:22Z), which pins
`specs/1170/design.json` (`planned_by: human-fallback`, 2026-10-30), `LAYERS.md`, and the code on `dev` `bc78349`.
"Delivered" cites code on `dev`. "Planned, not built" cites a PR or an issue body. On 2026-11-07 the end of the mission
closed the PRs of the planned parts unmerged; their branches are kept
([05-implementation-plan.md](05-implementation-plan.md), "Closed unmerged").

## Architecture in one paragraph

The Feature adds card kinds to the one inbox (`internal/cli/inbox.go`, `inboxKinds` and `inboxOrder`). Each card
approves through the verb that already owns the write: a Baseline card through the same path as
`aayu ready <f> --part <part>`, an Approve to build card through `approveToBuild` in `internal/cli/readyapprove.go`,
and the inbox session asks for one device code (`internal/cli/presence.go`, `presenceCheck`). The mission plan
(`missions/<f>/plan.json`) gains a final BUGS milestone. The evening reads proof packs from the ledger and writes the
person's review to `missions/<f>/reviews.jsonl`. `aayu grill --edit` writes a checked body through the actions log.
The design's contracts are the seven lines of `design.json` `contracts`, one per Story #1172 to #1179.

## Layers

| Layer (`LAYERS.md`) | In the design? | Touched by delivered code | Touched by closed PRs (branches kept) |
|---|---|---|---|
| L02 Interaction / API boundary (`internal/cli/**`, `internal/mcp/**`) | yes (`design.json` `layers`) | every Story PR (`internal/cli/inbox*.go`, `mission*.go`, `grill.go`; `inboxovernight.go` in [bc78349](https://github.com/rahulnakmol/agentic-ayudha/commit/bc78349)); `internal/mcp/schemas/grill.json` ([197ec9d](https://github.com/rahulnakmol/agentic-ayudha/commit/197ec9d)) | all |
| L03 Application / workflow (`internal/planning/**`, `internal/ready/**`, `internal/workitems/**`, `internal/verbs/**`, `internal/delegate/**`, `skills/**`, `loops/**`) | yes | [197ec9d](https://github.com/rahulnakmol/agentic-ayudha/commit/197ec9d) (`internal/planning/slice.go`, `internal/ready/ready.go`, `skills/grill/SKILL.md`); Bug fixes [932851d](https://github.com/rahulnakmol/agentic-ayudha/commit/932851d), [bed5524](https://github.com/rahulnakmol/agentic-ayudha/commit/bed5524), [7d3ec79](https://github.com/rahulnakmol/agentic-ayudha/commit/7d3ec79) | #1261 (`internal/delegate/brief.go`), #1270 (`loops/**`), #1273 (`skills/ready/SKILL.md`, still open) |
| L04 Domain / policy (`internal/verify/**`, `CONSTRAINTS.md`) | no | Bug fix [e39b5cb](https://github.com/rahulnakmol/agentic-ayudha/commit/e39b5cb) (`internal/verify/behaviour.go`) | #1249, #1270, #1282 (`CONSTRAINTS.md`) |
| L06 Contracts / integration (`internal/github/**`, `internal/notify`) | no | none | #1261, #1270 (`internal/github/fake/fake.go`); #1270 (`internal/notify/notify.go`) |
| L07 Operational data / state (`internal/config/**`, `internal/identity/**`) | no | Bug fix [26537bd](https://github.com/rahulnakmol/agentic-ayudha/commit/26537bd) (`internal/config/config.go`, `load.go`) | #1249, #1270, #1282 |
| L09 Platform / operations (`.github/**`) | no | none | #1261 (`.github/workflows/ci.yml`), #1270 (`.github/workflows/aayudha-loops.yml`), #1282 (`.github/CODEOWNERS`) |

L04, L07, and L09 are never layers (`internal/config/defaults.toml`, `never = ["L04", "L07", "L09"]`). A Story whose
PR touches a never layer needs the Engineering Manager's plan-time review (`internal/cli/ship.go`, the refusal that
names `aayu plan <story> --draft <file> --create`). A story plan must stay inside its Feature's design
(`internal/cli/plan.go`, the comment "A story plan stays inside its Feature's design; a bug fix may need another
layer", and `internal/planning/design.go`, `CheckInDesign`: "checkpoint … is outside the technical design of #…").
The design of #1170 holds only L02 and L03. So no plan for #1250, #1254, or #1281 can pass while the design stays as it is.
A new design cannot simply add layers: `CheckDesign` in `internal/planning/design.go` refuses a layer that is not in
the Feature's footprint, and `planFeature` in `internal/cli/plan.go` computes that footprint from the Feature's
Expected paths. #1170's Expected paths name ten files under `internal/cli/` (`inbox.go`, `ready.go`, `mission.go`, and others),
`internal/grill/**`, `internal/planning/**`,
`internal/ready/**`, `internal/ui/**`, `internal/verbs/registry.go`, `skills/grill/SKILL.md`,
`docs/guide/skills/grill.md`, and `GLOSSARY.md` (`specs/1170/ready.md`); `design.json` records the footprint as L02 and L03.
For #1250 this was the known block on PR #1261. For #1254 and #1281 it follows from the same code and their files
(inferred: no plan attempt for them is recorded on the ledger or in their PRs).

## Delivered parts, with code

| Contract (`design.json`) | Code on `dev` | Commit |
|---|---|---|
| Baseline card kind (#1172) | `internal/cli/inbox.go` `InboxBaseline`; `internal/cli/inboxbaseline.go` `baselineCards`, `baselineCard`, `baselineChanged`; parts `intent`, `spec`, `tech`, `ux` in `internal/ready/baseline.go` `Parts` (a UX card needs L01 in the footprint, `inboxbaseline.go` line 69) | [c858a80](https://github.com/rahulnakmol/agentic-ayudha/commit/c858a80) |
| Approve to build card kind (#1173) | `internal/cli/inbox.go` `InboxApprove`; `internal/cli/inboxapprove.go` `approveCard`, `approveItem`, `approveChanged`, `approveStories`; `internal/cli/readyapprove.go` | [eb51d1b](https://github.com/rahulnakmol/agentic-ayudha/commit/eb51d1b) |
| BUGS milestone (#1174) | `internal/cli/mission.go` `BugsMilestone`, `openBugs`, `withBugs`, `joinBugs`; BUGS must be last (`mission.go` line 664); `internal/cli/missiongate.go` `bugPacks` (detail `#%d merged with no proof pack`) | [322e2ac](https://github.com/rahulnakmol/agentic-ayudha/commit/322e2ac) |
| Morning order and "The day is set" (#1175) | `internal/cli/inbox.go` `inboxOrder`; `internal/cli/inboxday.go` `dayIsSet`, `daySet` | [2715ef2](https://github.com/rahulnakmol/agentic-ayudha/commit/2715ef2), test fix [4faab61](https://github.com/rahulnakmol/agentic-ayudha/commit/4faab61) |
| Evening card kind and reviews (#1176) | `internal/cli/inbox.go` `InboxEvening`; `internal/cli/inboxevening.go` `MissionReviewsPath` (`missions/%d/reviews.jsonl`), `eveningItems`, `eveningStories`, `hasEvidence`, `evidenceLink`, `inlineImage`, `notMerged`, `eveningAct`, `withoutAccept` | [188bbb0](https://github.com/rahulnakmol/agentic-ayudha/commit/188bbb0) |
| "The day is proven" (#1177) | `internal/cli/inboxevening.go` `dayTally`, `proves`, `cheer`; `internal/cli/inbox.go` `dayProven` calls `recordMilestone("day-proven:" + date)`; `internal/cli/look.go` `recordMilestone` writes the local `milestones.json`, not the ledger | [66397d3](https://github.com/rahulnakmol/agentic-ayudha/commit/66397d3) |
| Overnight card kind (#1178) | `internal/cli/inbox.go` `InboxOvernight` (in `inboxOrder` after `InboxEvening`); `internal/cli/inboxovernight.go` `OvernightPath` (`missions/overnight/<date>.json`), `overnightRuns`, `overnightAct`, `tickOvernight`, `overnightStep`, `overnightSpend`; `internal/cli/missiontick.go` calls `tickOvernight`; `internal/cli/missionnext.go` (`outside the day: starts at …`); `missiontick.go` `dueRetries` and `missionrecover.go` start nothing outside the day | [bc78349](https://github.com/rahulnakmol/agentic-ayudha/commit/bc78349) |
| `aayu grill --edit` (#1179) | `internal/cli/grill.go` `runGrillEdit` (refusal `#%d is Ready; its body is frozen`); `internal/planning/slice.go` `Paths` (comma-separated paths); `internal/ready/ready.go`, `internal/workitems/validate.go`; `internal/verbs/registry.go` (flag text) | [197ec9d](https://github.com/rahulnakmol/agentic-ayudha/commit/197ec9d) |

Delivered Bug fixes and their main code:

| Bug | Code on `dev` | Commit |
|---|---|---|
| #1182 | `internal/cli/readyapprove.go`, `internal/cli/missiongate.go` | [c64bccf](https://github.com/rahulnakmol/agentic-ayudha/commit/c64bccf) |
| #1193 | `internal/config/config.go`, `internal/config/load.go` (`[hosts] repos` in the user config), `internal/cli/mission.go` | [26537bd](https://github.com/rahulnakmol/agentic-ayudha/commit/26537bd) |
| #1207 | `internal/cli/missionnext.go`, `internal/cli/missionrecover.go`, `internal/cli/delegate.go` | [a17d469](https://github.com/rahulnakmol/agentic-ayudha/commit/a17d469) |
| #1208 | `internal/cli/build.go`, `internal/cli/delegate.go`, `internal/delegate/brief.go`, `receivers.go`, `state.go` | [932851d](https://github.com/rahulnakmol/agentic-ayudha/commit/932851d) |
| #1210 | `internal/planning/slice.go`, `LAYERS.md`, `internal/verbs/registry.go` | [bed5524](https://github.com/rahulnakmol/agentic-ayudha/commit/bed5524) |
| #1211 | `internal/agentenv/cloud.go`, `status.go`, `internal/cli/installenv.go`, `internal/cli/delegate.go` | [7d3ec79](https://github.com/rahulnakmol/agentic-ayudha/commit/7d3ec79) |
| #1226 | `internal/verify/behaviour.go`, `hosts/verify-skill/node/control.mjs`, `internal/cli/missiongate.go` | [e39b5cb](https://github.com/rahulnakmol/agentic-ayudha/commit/e39b5cb) |
| #1230 | `internal/cli/missiongate.go`, `missionask.go`, `mission.go` | [0c4abee](https://github.com/rahulnakmol/agentic-ayudha/commit/0c4abee) |

## Ledger records

| Record | Written by | State |
|---|---|---|
| `specs/<f>/baselines/<part>-v<k>.json` | Baseline card or `aayu ready <f> --part` | delivered; #1170 has intent v1, spec v1-v3, tech v1 |
| `specs/<f>/approved.json` | Approve to build card or `aayu ready <f>` | delivered; #1170 has version 3, 10 stories, `agents` 10, budget 600 USD |
| `missions/<f>/plan.json` | Approve to build; the mission | delivered; #1170's plan has `milestones` MS1, MS2, BUGS, `feature_done: "BUGS"`, `gate_fixes` (8 entries), `controls` (the last is `end`, 2026-11-07T12:35:56Z), and `state` `ended (founders regroup: v0.4 paused)` |
| `missions/<f>/gates.jsonl` | `aayu mission gate` | delivered; #1170 has 6 MS1 runs (2 passed, 4 failed) |
| `missions/<f>/reviews.jsonl` | Evening card accept | delivered in code (`MissionReviewsPath`); `missions/1170/` has no such file on `origin/ledger` |
| `missions/overnight/<date>.json` | Overnight card (#1178) | delivered in code (`bc78349`, `inboxovernight.go` `OvernightPath`); `origin/ledger` has no `missions/overnight/` file yet |
| `notify/outbox.jsonl` | Requests that wait for a window (#1254) | planned, not built (PR #1270 body; PR closed unmerged) |
| `missions/<f>/plan.json` `rebases` | ship's rebase refusal (#1234) | planned, not built (PR #1236 body, `MissionRebase`; PR closed unmerged) |

## Config keys

| Key | Value on `dev` | Role | Source |
|---|---|---|---|
| `[time] business_hours` | `Mon-Fri 09:00-18:00` | evening review time; the end of each overnight run and the day that `mission next` starts in (#1178) | `internal/config/defaults.toml` |
| `[time] tz` | `Europe/London` (default) | the day's zone; #1254 sets `Asia/Kolkata` in `.aayudha/config.toml` | `defaults.toml`; PR #1270 |
| `[time] windows` | not on `dev` | two person windows, default `["10:30-11:30", "18:30-19:30"]` | planned, not built (PR #1270) |
| `[mission] max_hours` | 48 | time limit before new starts wait | `CONSTRAINTS.md` |
| `[mission] max_no_go` | 3 | failed gate runs before the mission stops | `CONSTRAINTS.md` |
| `[budget] per_story_usd` | 60.0 | Approve to build budget per story | `CONSTRAINTS.md` |
| `[ui] plain` | — | words only, no motion (#1177 criterion 3) | `aayu inbox --help` (`--plain`) |
| `[autonomy] always_person` | 21 entries (ship files, `internal/config/`, workflows, `go.mod`, `loops/build-ready.toml`, and others) | which PRs wait for a person | `.aayudha/config.toml`; #1281 would cut it (PR #1282) |
| `[autonomy] cross_family` | not on `dev` | freed paths that need a cross-family review | planned, not built (PR #1282 body) |

## Planned, not built

- The overnight worker (accepted follow-ups F1, F2, and F4 of PR #1238): no worker runs an overnight run yet; the
  tick only records start and stop (`inboxovernight.go` `tickOvernight`; PR #1238 RAID, Goldfish finding 1). Spend
  reads only the local actions log (`overnightSpend`; Goldfish finding 4). No design for the worker exists in a
  source.
- The night-retry fix (accepted CodeRabbit follow-up of PR #1238): `dueRetries` in `internal/cli/missiontick.go`
  returns early outside the day (`if !day.Within(now)`), so a retry that came due at night can stay `failed` after
  the day opens (CodeRabbit comment on line 327).
- Draft-first CI (#1250, PR #1261, closed unmerged): `.github/workflows/ci.yml` runs on `ready_for_review` and skips drafts;
  `internal/delegate/brief.go` says draft first; `internal/cli/ship.go` refuses a draft; `internal/cli/shipproof.go`
  `requiredChecks` drops skipped runs from before the last ready event.
- Person windows (#1254, PR #1270, closed unmerged): `internal/config/time.go`, `internal/cli/windows.go`, `internal/cli/missionask.go`,
  `internal/notify/notify.go` outbox, and changes to the 13 Amp loop definitions in `loops/amp/*.json`.
- Merge on proof for every agent PR (#1281, PR #1282, closed unmerged): a shorter `[autonomy] always_person`, a new
  `[autonomy] cross_family`, and a `.github/CODEOWNERS` that protects only the policy files.

## Key decisions

| Decision | What it sets | Source |
|---|---|---|
| [#1168](https://github.com/rahulnakmol/agentic-ayudha/issues/1168) | The founder's day: one morning session, proven by evening, Bugs last, author overnight | #1170 Summary; closed `status:done` |
| [#1255](https://github.com/rahulnakmol/agentic-ayudha/issues/1255) | Option 2: every agent PR merges on proof at any risk and hour; only `always_person_config` and secret or environment workflows stay with a person | issue body, "Decision" section; `constrains #1170` |
| [#1027](https://github.com/rahulnakmol/agentic-ayudha/issues/1027) | Inside an approved mission the approval replaces the per-PR person (owned outside this Feature) | #1170 Scope, Out; PR #1199 names it for the authority change |
| Merge order by milestone | A later milestone's PR waits for every earlier gate ("M9 A: build ahead, merge in order") | `internal/cli/missionstop.go` `missionHold` comment (Story #1002; Decisions #915 and #974) |
