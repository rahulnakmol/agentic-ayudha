# #1170 Sequencing of the pending work

This is a plan, not a record. It orders into waves the 13 PRs that the end of the mission closed unmerged on
2026-11-07, the open PR #1273, the 5 Bugs with no PR, and the four accepted follow-ups of PR #1238
([05-implementation-plan.md](05-implementation-plan.md)). A lane is one Story, one Bug, or one gate run. File lists
come from `gh pr view <n> --json files` for the PRs, and from the issue Summary for Bugs with no PR (marked "from the
issue"). Test files and `.aayudha/evidence/<n>.toml` are left out where they do not overlap.

Each lane below that names a closed PR means: rebuild that PR, or reopen it from its kept branch (the branch and head
are in [05-implementation-plan.md](05-implementation-plan.md), "Closed unmerged"). Then rebase it onto `dev`
`bc78349` or later, run its checks and inspect again, and ship it. `bc78349` (PR #1238) changed `internal/cli/inbox.go`,
`missionnext.go`, `missiontick.go`, `missionrecover.go`, `inboxevening.go`, `mission_test.go`, and `inboxday_test.go`
(`git show --stat bc78349`). The kept branches of #1199, #1236, #1249, #1262, #1265, and #1270 change some of these
files, so expect conflicts in their rebase (from `gh pr view <n> --json files`).

## Rules that shape the order

1. The mission ended on 2026-11-07T12:35:56Z (`missions/1170/plan.json`, control `end`). An ended mission holds
   nothing at ship (`internal/cli/missionstop.go`, `missionHold`: "a mission that has not started, or has ended,
   holds nothing"), so MS2 does not gate the BUGS PRs now. It also starts nothing (`internal/cli/missionnext.go`:
   "The mission of #%d %s. Nothing starts."). To build with agents again, a person must approve #1170 to build again
   and run `aayu mission start 1170` (Wave 0, P7 and P8). Then the hold comes back with the plan's milestones: MS1
   passed, MS2 and BUGS still say `waits` (`missions/1170/plan.json`), so BUGS waits for MS2 again (inferred from the
   same code). Build ahead is allowed: the hold is on the merge, not the build ("M9 A: build ahead, merge in order",
   `missionHold` comment). This plan keeps milestone order, so it works with a restarted mission.
2. A Story plan must stay inside #1170's design, L02 and L03 (`specs/1170/design.json`;
   `internal/planning/design.go`, `CheckInDesign`). #1250, #1254, and #1281 touch never layers (L04, L07, L09), and
   ship needs a plan-time review for them (`internal/cli/ship.go`). So the Feature's Expected paths and its design
   must change first (Wave 0, P6a and P6).
3. Approve to build again with `--milestones` resets a passed gate while Bug #1267 is open (#1267 Summary). With no
   `--milestones`, every gate is kept today (#1267 criterion 3: "as today"). The fix, PR #1276, was closed by the end. It is
   in BUGS, so under a running mission it cannot merge first (rule 1). While no mission runs, ship holds nothing, so
   #1276 could be reopened and merged before P7 (inferred from `missionHold`; the halt must be lifted first, rule 4).
   Else re-approve with no `--milestones` where the milestones need no change. If they must change
   (for example to place #1281), expect one more MS1 gate run. The last reset cost three runs
   (`missions/1170/gates.jsonl`, 2026-11-02T21:05Z to 2026-11-03T16:52Z).
4. The repository is halted (Actions variable `AAYUDHA_HALT`, set by @rahulnakmol at 2026-11-07T12:50:50Z), and
   every Amp loop schedule and GitHub subscription for it is paused (the pause of the loops and subscriptions is
   unverified here). A person lifts the halt (`aayu resume`, a
   Founder verb, AGENTS.md RACI) before any agent lane runs. `mission.max_hours` is 48 (`CONSTRAINTS.md`); the ended
   mission had used its 96 h (#1170 comment `aayudha:mission-time`). How a restarted mission counts its hours is not
   verified here.
5. Merges into `dev` are one at a time. Ship stops when `dev` moves while it judges a PR (`internal/cli/ship.go`,
   "moved from … while ship judged the PR"). After #1236 (#1234) merges, ship also refuses a head that does not
   contain the dev tip, so each later PR rebases and runs its gate again before it merges.
6. Two lanes in one wave edit no common file. Where two PRs must touch one file, they are in different waves.

## Shared hot files

| File | PRs and Bugs that change it |
|---|---|
| `internal/cli/inbox.go` | #1270, #1249, #1265, #1199, #1201 (from the issue); changed on `dev` by `bc78349` |
| `internal/cli/ship.go`, `ship_test.go` | #1261, #1270, #1214, #1220, #1236; `ship_test.go` also #1282 |
| `internal/cli/shipproof.go`, `shippass.go` | #1261, #1220, #1282 |
| `internal/cli/mission.go` | #1276, #1262, #1236 |
| `internal/cli/missionnext.go` | #1270, #1249, #1236, #1280 (from the issue); changed on `dev` by `bc78349` |
| `internal/cli/missiontick.go` | #1270, the night-retry follow-up of #1238 (`dueRetries`); changed on `dev` by `bc78349` |
| `internal/cli/missiongate.go`, `missiongate_test.go` | #1270, #1276, #1275, #1262, #1274 (from the issue) |
| `internal/cli/missiontime.go` | #1270, #1249, #1262 |
| `internal/cli/missionstop.go` | #1236, #1280, #1274 (both from the issues) |
| `internal/cli/readyapprove.go` | #1259, #1262, #1265, #1199 |
| `internal/cli/mission_test.go`, `cli_test.go` | #1270, #1262, #1199; `cli_test.go` also #1249; `mission_test.go` changed on `dev` by `bc78349` |
| `internal/config/*` and the generated `CONSTRAINTS.md` | #1270, #1249, #1282 (`go generate ./...` renders `CONSTRAINTS.md` from `internal/config/defaults.toml`, AGENTS.md) |
| `internal/verbs/registry.go` and its generated files (`skills/*/SKILL.md` frontmatter, `internal/mcp/schemas/*.json`, `docs/README.md`) | #1249 (regenerates `skills/mission/SKILL.md` and `internal/mcp/schemas/mission.json`) |
| `.aayudha/config.toml` | #1270, #1282 |
| `internal/github/fake/fake.go` | #1261, #1270 |
| `internal/delegate/brief.go` | #1261, #1256 (from the issue) |
| `ratchets.toml` | any PR whose coverage moves the floor (AGENTS.md, coverage ratchet) |

## Wave 0: the person's decisions (no agent lanes)

All of these are human-only. They fit in one or two person sittings.

| # | Step | Why | Command or place |
|---|---|---|---|
| P0 | Lift the repository halt | rule 4 | `aayu resume` (human-only) |
| P1 | Say which rule governs overnight merges: #1170 Scope ("no merge overnight") or Decision #1255 ("any hour") | #1281 depends on it; #1178's card merged without a recorded ruling (the evidence does not show one) | a comment or a new spec baseline; this folder does not know the person's answer |
| P2 | Done on 2026-11-07: @rahulnakmol accepted #1238's Goldfish P1 findings F1, F2, and F4 and CodeRabbit's night-retry bug as follow-ups, and #1238 merged as `bc78349`. What is left: file each as a Story or Bug under #1170 (or another Feature) | the follow-ups are not filed as issues | `aayu grill story` or `aayu grill bug`, then Ready |
| P3 | Rule on #1270's P1 findings 3.7 and 3.11 | PR #1270 RAID: "Recommended: accept as a follow-up Bug" | the same |
| P4 | Look at #1199 (authority change, reduced criterion 4) and at the three CodeRabbit Major findings of #1220 | both PRs leave these to a person | `aayu inspect <pr> --dismiss <id> --reason …` is human-only |
| P5 | Choose #1281's home: a Bug under #1170, a new Feature, or #1170 approved again with #1281 | PR #1282 RAID | — |
| P6a | Edit #1170's Expected paths to add the L04, L06, L07, and L09 paths of #1250, #1254, and #1281 | A design may name only layers in the Feature's footprint, which comes from its Expected paths (`internal/planning/design.go`, `CheckDesign`: "layer %s is not in the footprint"). The end of the mission ended its lock (`internal/mcp/schemas/mission.json`, `end`: "the lock ends"), so no Locked card applies now. The body is still Ready (`specs/1170/ready.md`), and `aayu grill --edit` refuses a Ready issue (`internal/cli/grill.go` `runGrillEdit`). The edit needs a new Ready snapshot. `intent-v1.json` pins the body hash, so an intent v2 is likely needed too (inferred). The path to edit a Ready Feature body is unverified | then `aayu ready 1170` |
| P6 | Approve a tech baseline v2 whose design covers L04, L06, L07, and L09. An agent drafts it first: `aayu plan 1170 --draft <file> --create` writes `specs/1170/design.json` (`internal/cli/plan.go`, `planFeature`), and `aayu ready 1170 --part tech --request` drafts the baseline that pins it (inferred flow: `tech-v1.json` pins `design.json` by hash) | rule 2 | Baseline card in `aayu inbox` |
| P7 | Approve to build again (approval version 4), with #1281 if P5 says so | the mission ended; the stories and design changed | Approve to build card in `aayu inbox`; with `--milestones`, expect MS1's gate to reset (rule 3) |
| P8 | Start the mission again | rule 1: an ended mission starts nothing; `missionExtend` (`internal/cli/missiontime.go`) does not change the plan's `state` | `aayu mission start 1170` |

Check P7 and P8 with care. On `dev`, Approve to build again on a plan that has a mission keeps the old plan's
`started_by`, tool, budget, and controls, and the approval keeps its Kanha parent (`internal/cli/readyapprove.go`, the
`old != nil` branch). It does not copy the plan's `state`. `aayu mission start` refuses a plan with `started_by` set
and an approval with a parent ("The mission of #%d started at …", `internal/cli/mission.go` `startable`). So how a
re-approval and a new start act on the ended plan of #1170 is unverified: it is inferred from the code, not tested.
`aayu mission show 1170` after P7 shows the state.

On 2026-11-07 the person ran `aayu mission stop 1170 --end`. As its help text says, it closed each open PR of the
mission: the 13 PRs in [05-implementation-plan.md](05-implementation-plan.md), "Closed unmerged".

## Wave 1: MS1 again, MS2's first PR, and two new fixes (4 lanes)

| Lane | Work | Files | Person step |
|---|---|---|---|
| 1A | Only if P7 reset MS1: an MS1 gate run on the dev tip, and a gate fix PR per failed check | none, or `skills/retro/SKILL.md` for a retro fix (as #1229, #1233, #1271, #1284) | `aayu mission gate 1170 --once-more` after `max_no_go` failures |
| 1B | Fix the night-retry follow-up of #1238 (filed in P2): a retry due outside the day stays due after the day opens | `internal/cli/missiontick.go` (`dueRetries`), its tests | P2 (file it) |
| 1C | #1261 (#1250), from its kept branch: `aayu plan 1250 --draft <file> --create`, rebase, ship | `.github/workflows/ci.yml`, `workflows_test.go`, `internal/cli/ship.go`, `ship_test.go`, `shipproof.go`, `internal/delegate/brief.go`, `internal/github/fake/fake.go`, `kanha_test.go` | `aayu plan 1250 --approve`; ship from `aayu inbox` (protected path `.github/`, and `ci.yml` names `live-tests`) |
| 1D | Build #1283 (Ready first) | `evals/cases/retro/**`, `internal/evals/**` (from the issue) | none known |

1B and 1C share no file. #1178 merged as `bc78349`, so MS2 holds only #1250 and #1254 now. #1283 makes the retro cases independent of the real date (#1283 Summary); it could also
serve as a gate fix for 1A if the retro evals fail again (unverified that the gate would accept it as a fix).

## Wave 2: the rest of MS2 (3 lanes)

| Lane | Work | Files | Person step |
|---|---|---|---|
| 2A | #1270 (#1254), from its kept branch: rebase onto `bc78349`, 1B, and 1C, the P3 outcome, `aayu plan 1254 --draft <file> --create`, mark ready, review, ship | 65 files: `internal/cli/inbox.go`, `missionnext.go`, `missiontick.go`, `missiontime.go`, `ship.go`, `windows.go`, `decide.go`, `internal/config/*`, `internal/notify/*`, `loops/**`, `.aayudha/config.toml`, `CONSTRAINTS.md`, `.github/workflows/aayudha-loops.yml`, `internal/github/fake/fake.go` | P3; `aayu plan 1254 --approve`; ship (protected paths and the narrow set); then `aayu install --loops --host amp --apply` (PR RAID) |
| 2B | Build #1256 (depends on #1250, merged in 1C) | `internal/delegate/brief.go`, its tests (from the issue) | none known |
| 2C | MS2 gate run after 2A merges, and gate fixes | as 1A | as 1A |

PR #1270 is the largest PR and touches most hot files, so it goes alone after 1B and 1C. 2A and 1B both change
`internal/cli/missiontick.go`, so 1B merges first.

## Wave 3: BUGS, first set (6 lanes)

Starts when MS2's gate passes, if the mission was started again (rule 1). With no mission, ship holds nothing, but
the waves still follow the hot files.

| Lane | Work | Files |
|---|---|---|
| 3A | #1276 (#1267), from its kept branch: rebase and ship | `internal/cli/mission.go`, `missiongate_test.go` |
| 3B | #1214 (#1206), from its kept branch: rebase and ship | `internal/cli/ship.go`, `ship_test.go` |
| 3C | #1259 (#1251), from its kept branch: rebase and ship | `internal/cli/readyapprove.go`, `readycover.go` and tests |
| 3D | #1249 (#1248), from its kept branch: rebase, `go generate ./...`, ship | `internal/cli/missiontime.go`, `missionnext.go`, `inbox.go`, `cli_test.go`, `internal/config/*`, `CONSTRAINTS.md`, `internal/verbs/registry.go`, `skills/mission/SKILL.md`, `internal/mcp/schemas/mission.json` |
| 3E | #1283, ship | `evals/**`, `internal/evals/**` |
| 3F | #1256, ship | `internal/delegate/brief.go` |

Person steps: a ship for #1214 (narrow set `ship.go`); a plan review and ship for #1249 (L04, L07; protected
`internal/config/defaults.toml`, `CONSTRAINTS.md`, `skills/`).

## Wave 4: BUGS, second set (5 lanes)

| Lane | Work | Files |
|---|---|---|
| 4A | #1275 (#1268), from its kept branch: rebase and ship | `internal/cli/missiongate.go`, `missionask.go`, `missiongate_test.go` |
| 4B | #1220 (#1192), from its kept branch: rebase and ship | `internal/cli/ship.go`, `ship_test.go`, `shipproof.go`, `shippass.go`, `docs/guide/how-merge-gate.md`, `docs/guide/how-trust.md` |
| 4C | #1273 (#1251 skill text, still open), after #1259 | `skills/ready/SKILL.md` |
| 4D | #1265 (#1253), from its kept branch: rebase and ship | `internal/cli/inbox.go`, `inboxapprove.go`, `inboxbaseline.go`, `inboxtui.go`, `inboxwhy.go`, `inboxwork.go`, `readyapprove.go` |
| 4E | Build and ship #1280 (Ready first) | `internal/cli/missionstop.go`, `missionnext.go`, `missionwake.go` (from the issue) |

Person steps: P4 and a ship for #1220 (narrow set); `--solo-bypass` for #1273 (protected `skills/`).

## Wave 5: BUGS, third set (3 lanes)

| Lane | Work | Files |
|---|---|---|
| 5A | #1262 (#1252), from its kept branch: rebase and ship | `internal/cli/mission.go`, `mission_test.go`, `missiongate.go`, `missiontime.go`, `readyapprove.go`, `kanha_test.go` |
| 5B | Build and ship #1201 | `internal/decide/decide.go`, `internal/cli/inbox.go` (from the issue: `SupersededMarker`, the Decision card) |
| 5C | #1282 (#1281), from its kept branch, if P5 placed it here | `.aayudha/config.toml`, `.github/CODEOWNERS`, `internal/config/*`, `CONSTRAINTS.md`, `internal/cli/shipproof.go`, `shippass.go`, `ship_test.go`, `inboxaudit.go` |

Person steps: a ship for #1282 (CODEOWNERS and `[autonomy]`). If P5 places #1281 before BUGS instead, merge it right
after Wave 2: then #1214, #1220, and #1236 would merge on proof with an audit card, not a person's ship (PR #1282
body). #1282 also needs dev's `aayu` to know `[autonomy] cross_family` (PR #1282, blocker 2).

## Wave 6: BUGS, fourth set (2 lanes)

| Lane | Work | Files |
|---|---|---|
| 6A | #1236 (#1234), from its kept branch: rebase and ship | `internal/cli/ship.go`, `ship_test.go`, `mission.go`, `missionnext.go`, `missionstop.go`, `docs/guide/how-merge-gate.md` |
| 6B | #1199 (#1191), from its kept branch: fix inspect's `fix` finding, rebase (26 behind `884d81b`), ship | `internal/cli/inbox.go`, `presence.go`, `ready.go`, `ready_early.go`, `readyapprove.go`, `inspect.go`, `env.go`, `loops.go`, `mission_test.go`, `cli_test.go`, `internal/agentenv/*` |

Person steps: a ship for #1236 (narrow set `ship.go`); P4 and `--solo-bypass` for #1199 (`presence.go` is in the
narrow set). #1236 goes late on purpose: after it, every PR must contain the dev tip (rule 5).

## Wave 7: the last Bug and the Feature gate (2 lanes)

| Lane | Work | Files |
|---|---|---|
| 7A | Build and ship #1274 (after #1262 and #1199) | `internal/cli/presence.go` callers: `missionstop.go`, `missionrecover.go`, `missiongate.go`, `ready.go`, `ready_early.go`, `scope.go`, `inspect.go` (from the issue) |
| 7B | BUGS gate: a proof pack per Bug on the ledger (`internal/cli/missiongate.go`, `bugPacks`), then gate fixes | none |

When BUGS passes, the Feature is done (`missions/1170/plan.json`, `complete_when`).

## Why fewer than 10 lanes

The widest wave has 6 lanes. Four things keep it below 10:

1. Most of the work is already built. 13 closed PRs wait to be rebuilt or reopened, #1273 is open, and merges into
   `dev` go one at a time (rule 5).
2. With a restarted mission, the milestone hold stops every BUGS PR until MS2 passes (rule 1), so Waves 1 and 2 hold
   only MS2's two open Stories and two small fixes.
3. The closed PRs share hot files (`inbox.go`, `ship.go`, `mission.go`, `missiongate.go`, `readyapprove.go`), so
   rule 6 splits them into waves.
4. Only 5 Bugs need new code, each effort S (labels).

The unsliced work in [05-implementation-plan.md](05-implementation-plan.md) has no Story yet: the accepted #1238
follow-ups F1, F2, and F4 (one overnight worker that runs each run, stops it at its bound, and reads shared spend) and
the two #1254 follow-up Bugs. Once filed and approved, they can run as more lanes in Waves 3 to 7. F1 needs
`loops/build-ready.toml`, which is in `[autonomy] always_person` (`.aayudha/config.toml`), so it needs a person's
ship. F2 and F4 touch `internal/cli/inboxovernight.go` (`tickOvernight`, `overnightSpend`) and, for F4,
`internal/cli/build.go` `buildCost` (PR #1238 RAID). Their other files are not known yet.
