# Sequencing: Feature #889

This plan builds the pending work of [05-implementation-plan.md](05-implementation-plan.md): 12 Stories, 4 open
Bugs, the rest of Bug #1166, and the 5 Bugs whose PRs are in review. It uses at most 10 implementation threads at
once. It is a plan, not a record: nothing in it is built.

## Rules this plan follows

- Depends-on links come from the issue bodies and `specs/889/baselines/spec-v1.json` (the `depends_on` of each
  pinned story).
- Two lanes in one wave edit no common file, except where a merge order is named.
- The files per lane come from the issue bodies (Environment, Notes, Review guidance) and from the code on `dev` at
  `884d81b`. A lane may need a test file next to each file it names (`<file>_test.go`); these are not listed again
  unless another lane also needs them.
- Every Story or Bug PR merges only through `aayu ship` (AGENTS.md, "Branches, PRs, commits").

## Wave 1: start now (8 lanes)

Every dependency of these lanes is merged, or the lane only finishes an open PR.

| Lane | Work | Files or packages | Notes |
|---|---|---|---|
| 1 | Finish [#1138](https://github.com/rahulnakmol/agentic-ayudha/issues/1138) in PR [#1260](https://github.com/rahulnakmol/agentic-ayudha/issues/1260) | `internal/cli/kanha.go`, `kanhalocal.go`, `build.go`, `kanhaevents_test.go`, `ship_test.go` | Fix the red `build` and the floor miss first. Merges before lane 2 (both touch `ship_test.go`) |
| 2 | Proof-land twins, one thread, in order: [#1161](https://github.com/rahulnakmol/agentic-ayudha/issues/1161) (PR [#1183](https://github.com/rahulnakmol/agentic-ayudha/issues/1183)), then [#1204](https://github.com/rahulnakmol/agentic-ayudha/issues/1204) (PR [#1277](https://github.com/rahulnakmol/agentic-ayudha/issues/1277)), then [#1203](https://github.com/rahulnakmol/agentic-ayudha/issues/1203) | `internal/cli/home.go` (`gateSuites`, `mergedRunOf`, `hadHead`), `internal/github/fake/fake.go`, `internal/cli/ship_test.go` | All three change the same functions, so one thread does them in series. #1203 needs `hadHead`, which only PR #1183 adds. Rebase on lane 1 |
| 3 | Finish [#1198](https://github.com/rahulnakmol/agentic-ayudha/issues/1198) in PR [#1202](https://github.com/rahulnakmol/agentic-ayudha/issues/1202) | `internal/config/config.go`, `load.go`, `config_test.go`, `docs/guide/how-config-and-audit.md` | A person reviews and ships it (narrow set) |
| 4 | Rest of [#1166](https://github.com/rahulnakmol/agentic-ayudha/issues/1166): the halted case | `evals/cases/mission/halted/**` | Starts only after a person accepts the eval-bar waiver (PR #1181 RAID log) |
| 5 | [#1104](https://github.com/rahulnakmol/agentic-ayudha/issues/1104) 10-step build loop | `internal/delegate/brief.go` (`Brief`), `internal/delegate/delegate_test.go`, `evals/cases/build/ten-step-loop/**` (new), `internal/evals` fixtures test | Unblocks #1109, #1110, #1111 |
| 6 | [#1134](https://github.com/rahulnakmol/agentic-ayudha/issues/1134) lock ends at the last gate | `internal/cli/missiongate.go` (`missionGate`), `missionlock.go` (`endLock`), `missiongate_test.go` | PR [#1275](https://github.com/rahulnakmol/agentic-ayudha/issues/1275) (Bug #1268, Feature #1170) also changes `missiongate.go` and its test. It was closed unmerged on 2026-11-07 when the mission of #1170 ended; branch `fix/gate-count-since-pass` at `9e8b72b` kept (`_data/closed-prs.tsv`). If a person reopens or rebuilds #1268, agree the merge order first |
| 7 | [#1128](https://github.com/rahulnakmol/agentic-ayudha/issues/1128) send a follow-up once | `internal/cli/delegate.go` (`delegation.followUp`, `Env.reviewLoop`), `internal/delegate/state.go`, `internal/cli/delegate_test.go` | Draft PR [#1278](https://github.com/rahulnakmol/agentic-ayudha/issues/1278) (no Refs) also changes `delegate.go`; agree the order with its owner |
| 8 | [#1154](https://github.com/rahulnakmol/agentic-ayudha/issues/1154) step 1: profile only | no file changes: `go test -json ./internal/cli`, sorted by Elapsed | The issue's checkpoint 1. The code change waits for the last wave |

Why not 10: the other pending items wait on a lane above, on another Feature, or on a person.

## Wave 2: after lane 1 and lane 7 merge (3 lanes)

| Lane | Work | Files or packages | Notes |
|---|---|---|---|
| 1 | [#1139](https://github.com/rahulnakmol/agentic-ayudha/issues/1139) untrusted events in `mission next` and `recover` | `internal/cli/missionnext.go` (`emitWorkList`), `missionrecover.go` (`emitRecover`), `missiontick.go` (`missionEvents`, read only) | Depends on #1138. Merges before lane 2 (both touch `missionnext.go`) |
| 2 | Finish [#1266](https://github.com/rahulnakmol/agentic-ayudha/issues/1266) in PR [#1269](https://github.com/rahulnakmol/agentic-ayudha/issues/1269) | `internal/cli/build.go`, `delegate.go`, `delegatehost_test.go`, `missionnext.go`, `internal/delegate/receivers.go`, `internal/delegate/state.go`, `skills/mission/SKILL.md` | Rebase after lane 1 of this wave and after #1128. `skills/mission/SKILL.md` is a protected path: the founder's solo-mode bypass |
| 3 | [#1110](https://github.com/rahulnakmol/agentic-ayudha/issues/1110) real-run journeys (v0.4.1) | `journeys/**` (proposed, new), `internal/verify/**`, `internal/cli/missiongate.go` (gate checks `journey:<name>`) | Depends on #1104 and #1001. Wave 1 lane 6 already changed `missiongate.go`, so rebase on it |

Why not 10: `missionnext.go`, `build.go`, and `delegate.go` are each wanted by several of the remaining lanes.

## Wave 3: after wave 2 (2 lanes)

| Lane | Work | Files or packages | Notes |
|---|---|---|---|
| 1 | [#1197](https://github.com/rahulnakmol/agentic-ayudha/issues/1197) untrusted receiver errors in build, wake, and tick | `internal/cli/build.go`, `delegate.go`, `missionwake.go`, `missiontick.go` | Depends on #1139 |
| 2 | [#1126](https://github.com/rahulnakmol/agentic-ayudha/issues/1126) local wake loop | `internal/cli/installloops.go`, `internal/loops/**`, `internal/cli/kanhalocal.go` (check-in), `missionnext.go` (starts nothing when the machine is off), `mission.go` (`missionShow` line) | Depends on #1012 and #1009 (merged). The scheduler per tool is open (issue Notes). If the install help changes, `internal/verbs/registry.go` too |

## Wave 4: after wave 3 (2 lanes, v0.4.1)

| Lane | Work | Files or packages | Notes |
|---|---|---|---|
| 1 | [#1109](https://github.com/rahulnakmol/agentic-ayudha/issues/1109) red runs in proof packs | `internal/proof/pack.go` (`Pack`), `internal/verify/**`, `internal/cli/build.go` (`--red`), `internal/verbs/registry.go` and its generated files | Depends on #1104. Waits for wave 2 lane 3 on `internal/verify/**` |
| 2 | [#1111](https://github.com/rahulnakmol/agentic-ayudha/issues/1111) `beyond-ask` finding and value record | `internal/inspect/**` (`Finding`), `internal/cli/missiongate.go`, `mission.go` (`MissionMilestone`) | Depends on #1104, #1001, and Story [#985](https://github.com/rahulnakmol/agentic-ayudha/issues/985) (planned, another Feature) |

## Wave 5: after other Features deliver (2 lanes)

| Lane | Work | Files or packages | Notes |
|---|---|---|---|
| 1 | [#1010](https://github.com/rahulnakmol/agentic-ayudha/issues/1010) events record and progress comment | `internal/delivery/**` (created by Feature [#872](https://github.com/rahulnakmol/agentic-ayudha/issues/872)), `internal/cli/ship.go`, `build.go`, `kanha.go`, `missiontick.go` | Needs #872; criterion 2 needs `aayu ask` from Feature [#880](https://github.com/rahulnakmol/agentic-ayudha/issues/880). Merges before lane 2 (`ship.go`) |
| 2 | [#1030](https://github.com/rahulnakmol/agentic-ayudha/issues/1030) merge on proof inside a mission | `internal/cli/ship.go`, `shipproof.go`, `missionnext.go` (waits lines), `internal/config/defaults.toml` and `config.go` (`[risk]` keys), `CONSTRAINTS.md` (generated), inbox card files | Needs Stories #1028, #1029, #1083, and first a person's call on what is left after Decision #1255 and Story #1281 (draft PR [#1282](https://github.com/rahulnakmol/agentic-ayudha/issues/1282), which changes `shipproof.go` and `internal/config/`, was closed unmerged on 2026-11-07 when the mission of #1170 ended; branch `feat/1281-auto-merge-on-proof` at `6f2aae7` kept) |

## Wave 6: after #1010 (2 lanes)

| Lane | Work | Files or packages | Notes |
|---|---|---|---|
| 1 | [#1011](https://github.com/rahulnakmol/agentic-ayudha/issues/1011) pages only at milestone end, block, and mission end | `internal/notify/**`, `internal/delivery/**` (reader), `internal/cli/missiongate.go` (milestone end), `internal/delegate` (stub notifier tests) | Depends on #1010 and #1001 |
| 2 | [#1014](https://github.com/rahulnakmol/agentic-ayudha/issues/1014) feedback pass | new `internal/cli/missionfeedback.go` (proposed name), `internal/verbs/registry.go` (action `feedback`) and generated files, `skills/mission/SKILL.md`, inbox card files | Depends on #1010, #1001, #1013; needs `aayu triage --decide story --changes` (#879 S8, v0.4.1). Skill change: founder's bypass |

## Wave 7: last (2 lanes, in order)

| Lane | Work | Files or packages | Notes |
|---|---|---|---|
| 1 | [#1112](https://github.com/rahulnakmol/agentic-ayudha/issues/1112) milestone findings order the next milestone (v0.4.1) | `internal/cli/mission.go`, `missionnext.go` (order), `missiongate.go` (milestone end), `missions/<f>/learning/` record, inbox card files | Depends on #1014 and #1001 |
| 2 | [#1154](https://github.com/rahulnakmol/agentic-ayudha/issues/1154) faster `internal/cli` tests | `internal/cli/*_test.go`, shared fixtures (for example `newMissionWorld`), possibly a split into a new package | Run alone, after every other lane that edits `internal/cli` tests has merged; it touches the test files of every lane |

## Shared hot files that force serial merges

| File | Lanes that edit it | Why it is hot |
|---|---|---|
| `internal/verbs/registry.go` | W4-1, W6-2, maybe W3-2 | Generates `docs/README.md`, `LAYERS.md`, `docs/RACI.md`, `skills/*/SKILL.md` frontmatter, and `internal/mcp/schemas/*.json` (AGENTS.md); rebase and run `go generate ./...` after each merge |
| `internal/config/defaults.toml` | W5-2 | Generates `CONSTRAINTS.md`; protected path and in `always_person_config` (`.aayudha/config.toml`) |
| `internal/cli/missionnext.go` | W2-1, W2-2, W3-2, W5-2, W7-1 | Also changed by PR [#1249](https://github.com/rahulnakmol/agentic-ayudha/issues/1249) (Bug #1248, Feature #1170), closed unmerged on 2026-11-07 when the mission of #1170 ended; branch `story/1248-fix-mission-extend-refuses-before-the-ti` at `ec32cdc` kept |
| `internal/cli/missiongate.go` | W1-6, W2-3, W4-2, W6-1, W7-1 | Also changed by PR #1275, closed unmerged on 2026-11-07 when the mission of #1170 ended; branch `fix/gate-count-since-pass` at `9e8b72b` kept |
| `internal/cli/mission.go` | W3-2, W4-2, W7-1 | Also changed by PR [#1276](https://github.com/rahulnakmol/agentic-ayudha/issues/1276) (Bug #1267), closed unmerged on 2026-11-07 when the mission of #1170 ended; branch `fix/approve-keeps-gate` at `bcafd0f` kept |
| `internal/cli/build.go`, `internal/cli/delegate.go` | W1-1, W1-7, W2-2, W3-1, W4-1, W5-1 | Also changed by draft PR #1278 |
| `skills/mission/SKILL.md` | W2-2, W6-2 | Protected path (`.github/CODEOWNERS`: `/skills/`); also changed by PR #1249 (closed unmerged on 2026-11-07 when the mission of #1170 ended; branch kept) |
| `internal/github/fake/fake.go` | W1-2 | Most `internal/cli` tests use the fake |
| `ratchets.toml` | any lane | The coverage ratchet fails a PR when coverage holds well above the floor (AGENTS.md); each merge can move the floor, so rebase and rerun the ratchet |

## Steps that need a person

| Step | Lanes | Source |
|---|---|---|
| Ready snapshots for the planned Stories and Bugs (#1010, #1011, #1014, #1030, #1104, #1109 to #1112, #1126, #1128, #1134, #1154, #1197). Every existing snapshot of this Feature was written by @rahulnakmol | all story lanes | Ready snapshot comments on #1139 and #1157; `specs/<n>/ready.md` |
| Confirm the #1154 thresholds (300 s and 450 s) before Ready | W1-8, W7-2 | #1154, Open questions |
| Accept the eval-bar waiver for the halted case | W1-4 | PR #1181 RAID log; AGENTS.md, coverage ratchet and eval bars |
| Amend #1204 criterion 1 to match PR #1277's deviation | W1-2 | PR #1277 body |
| Reopen PR #1183 with `/aayu review again` after three judged rounds | W1-2 | AGENTS.md, "Respect the round cap"; comment on #1183 |
| Review and ship PR #1202 (`internal/config/` is in the narrow set) | W1-3 | `.aayudha/config.toml` `[autonomy] always_person`; comment on #1202 |
| Solo-mode bypass for `skills/mission/SKILL.md` | W2-2, W6-2 | `.github/CODEOWNERS`; AGENTS.md, "Protected paths" |
| Ship PRs that change `internal/cli/ship.go`, `shipproof.go`, or `internal/config/` | W5-1, W5-2 | `[autonomy] always_person` on `dev`; Decision #1255 and Story #1281 may change this set (its draft PR #1282 was closed unmerged on 2026-11-07 when the mission of #1170 ended) |
| Decide what is left of #1030 after Decision #1255, and record it in a new spec baseline version | W5-2 | `specs/889/baselines/spec-v1.json` pins #1030; `aayu ready <f> --part spec --request` |
| Put #1030's inbox card words into a UX baseline first | W5-2 | #1030 Notes; no UX baseline exists under `specs/889/` |
| Approve to build #889 and press Start, if the rest is to run as a #889 mission | before any mission run | No `specs/889/approved.json` or `missions/889/` on `origin/ledger`; `aayu mission start` is human-only (verb help) |
| Approve the feedback list (`aayu mission feedback <f> --approve`) and the next milestone's order at run time | after W6-2, W7-1 | #1014 C6; #1112 C3 |

## Assumptions in this plan

- One thread per lane; a lane that finishes an open PR reuses that PR's branch.
- The proposed paths `journeys/**` (#1110) and `internal/cli/missionfeedback.go` (#1014) are names for planning only.
  #1110 says "the plan may move them"; #1014 names no file.
- The cross-Feature draft PR #1278 is still open and lands in its own order; each lane above rebases on it. PRs
  #1249, #1275, #1276, and #1282 were closed unmerged on 2026-11-07 when the mission of #1170 ended (`_data/closed-prs.tsv`); their branches are kept, and nothing on
  `dev` waits for them.
