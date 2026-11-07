# Implementation plan: Feature #975

## Sources

- Sub-issues, states, and labels: `_data/subs.tsv` (feature 975).
- Merges: `_data/merge-refs.tsv` has no squash merge that refs any of the 20 Stories.
- Open PRs: `_data/open-prs.tsv` has no open PR that refs any of the 20 Stories.
- Depends-on: the "Links" section of each Story body. Core Feature states: GitHub API `sub_issues_summary`, and
  [03-technical-design.md](03-technical-design.md), "Core Features this one depends on".

## All Stories

All are `type:story`, `status:planned`, open, and "not started".

| # | Title | Effort | Risk | Depends on | Layers or paths (planned) | Evidence |
|---|---|---|---|---|---|---|
| [#937](https://github.com/rahulnakmol/agentic-ayudha/issues/937) | The day in areas: Now, Overdue, Ship, Answer, Decide, Ready, Housekeeping | M | low | #873, #885, #925 | L02: `inbox.go` (`inboxOrder`), `inboxtui.go` | not started |
| [#938](https://github.com/rahulnakmol/agentic-ayudha/issues/938) | Any seat asks you on an ANSWER card, and your answer reaches its thread | L | medium | #880, #881, #882, #871, #937 | L02: `inbox.go`, `inboxtui.go` | not started |
| [#939](https://github.com/rahulnakmol/agentic-ayudha/issues/939) | Not now, snooze, and not sure: put a card away and see when it comes back | M | low | #884, #937 | L02: `inboxtui.go` (s and ? menus), `inbox.go` | not started |
| [#940](https://github.com/rahulnakmol/agentic-ayudha/issues/940) | Overdue at 24 hours, and a recommendation applies at 48 hours only for a two-way ask | S | medium | #883, #937 | L02: `inboxtui.go` (banner), `inbox.go` | not started |
| [#941](https://github.com/rahulnakmol/agentic-ayudha/issues/941) | Refine a story to Ready in the inbox: what fails, ask the builder, and Make Ready | L | high | #871, #877, #880, #938 | L02: `inbox.go`, `inboxtui.go`, a new REFINE file (inferred) | not started |
| [#942](https://github.com/rahulnakmol/agentic-ayudha/issues/942) | Start a mission, and stop, resume, recover, or retry it from its card | M | high | #889, #941 | L02: `missionlock.go` (Start card), `mission.go`, `inbox.go` | not started |
| [#943](https://github.com/rahulnakmol/agentic-ayudha/issues/943) | aayu mission and the inbox's mission view: Shape and Deliver, each story's state, and each milestone's gate | L | medium | #889, #871, #937 | L02: `mission.go`, `inboxtui.go` (mission view); L03: `registry.go` (`--release`) | not started |
| [#944](https://github.com/rahulnakmol/agentic-ayudha/issues/944) | One story, end to end, on every card | S | low | #872, #885 | L02: `inboxtui.go` (`cardView`, L) | not started |
| [#945](https://github.com/rahulnakmol/agentic-ayudha/issues/945) | Review a plan and unblock a build from a card | M | high | #873, #877, #882 | L02: `inbox.go`, `build.go`, `delegate.go`, `plan.go` (inferred) | not started |
| [#946](https://github.com/rahulnakmol/agentic-ayudha/issues/946) | Ask for the stable release and publish copy from a card | M | high | #873 | L02: `inbox.go`, `release.go`, `tell` verb code (inferred) | not started |
| [#947](https://github.com/rahulnakmol/agentic-ayudha/issues/947) | Approve an incident's ops action from its card, with GitHub as the gate | M | high | #888, #937 | L02: `inbox.go`, `inboxtui.go` | not started |
| [#948](https://github.com/rahulnakmol/agentic-ayudha/issues/948) | Emergency stop for a tool incident, the budget pause, and resume | S | high | #873, #937 | L02: `inbox.go`, `inboxtui.go`, `halt.go`; L05: `internal/models/gateway.go` | not started |
| [#949](https://github.com/rahulnakmol/agentic-ayudha/issues/949) | A PR that a floor or an eval stops, or that loops, waits for you with three choices: waive, file a Bug and ship, or stop | L | high | #924, #936, #889, #873, #897 | L02: `inbox.go`, `inboxtui.go`, `decide.go`, `release.go`; L07: `defaults.toml` (`[inbox] loop_*`) | not started |
| [#950](https://github.com/rahulnakmol/agentic-ayudha/issues/950) | Accept, decline, or route Feedback from one card | S | low | #873 | L02: `inbox.go` | not started |
| [#951](https://github.com/rahulnakmol/agentic-ayudha/issues/951) | See each Feature in shaping, baseline its parts, and baseline again on drift | M | medium | #874, #937 | L02: `inboxbaseline.go`, `inbox.go` | not started |
| [#952](https://github.com/rahulnakmol/agentic-ayudha/issues/952) | Resolve the seats' findings on a baseline in a grill session, and see the record | S | medium | #875, #876, #951 | L02: `inbox.go`, `inboxbaseline.go` | not started |
| [#953](https://github.com/rahulnakmol/agentic-ayudha/issues/953) | Approve a Feature to build, knowing what it may spend | S | high | #877, #951 | L02: `inboxapprove.go`, `inbox.go` | not started |
| [#954](https://github.com/rahulnakmol/agentic-ayudha/issues/954) | Triage a bug from its repro report: hotfix, carve out a Feature, better repro, or not a bug | S | medium | #879, #937 | L02: `inbox.go`, `triage.go` | not started |
| [#955](https://github.com/rahulnakmol/agentic-ayudha/issues/955) | Acknowledge each story added or changed after Approve to build, or all of them on the record | M | high | #877, #953 | L02: `inbox.go`, `inboxapprove.go` | not started |
| [#956](https://github.com/rahulnakmol/agentic-ayudha/issues/956) | The delivery plan: waves, the critical path, an indicative time, and actual against estimate | M | low | #872, #886 | L02: `inboxtui.go` (p), `inbox.go` | not started |

The "Layers or paths" column is planned: from the Expected paths in issue #975, the Stories' Assumptions, and the
files on dev that hold today's version of the card ([03-technical-design.md](03-technical-design.md)). Paths marked
"inferred" are not named by a source.

## Delivered

No Story of #975 is merged into dev. Other Features built parts of what these Stories describe. They count for those
Features, not for #975:

| Part on dev | Commit | Story and Feature | #975 Story it starts |
|---|---|---|---|
| A waiting baseline draft is an inbox card | [c858a80](https://github.com/rahulnakmol/agentic-ayudha/commit/c858a80) (#1219) | #1172, Feature #1170 | #951 (BASELINE) |
| Approve to build is an inbox card | [eb51d1b](https://github.com/rahulnakmol/agentic-ayudha/commit/eb51d1b) (#1225) | #1173, Feature #1170 | #953 |
| The morning's cards come in decision order | [2715ef2](https://github.com/rahulnakmol/agentic-ayudha/commit/2715ef2) (#1227) | #1175, Feature #1170 | #937 (it changes the same order) |
| Mission start locks the Feature; a Start card | [3d42ff1](https://github.com/rahulnakmol/agentic-ayudha/commit/3d42ff1) (#1096) | #1013, Feature #889 | #942 |
| `aayu mission stop` and `resume`, with `--story` | [33aa6bf](https://github.com/rahulnakmol/agentic-ayudha/commit/33aa6bf) (#1093) | #1002, Feature #889 | #942, #949 (Stop) |
| `aayu mission recover` and `retry` | [bdd02c1](https://github.com/rahulnakmol/agentic-ayudha/commit/bdd02c1) (#1102) | #1003, Feature #889 | #942 |
| A gate-failed mission card | [3ece84e](https://github.com/rahulnakmol/agentic-ayudha/commit/3ece84e) (#1090) | #1001, Feature #889 | #943 C10 |
| A mission-time card with Extend and Stop | [2c6c6b0](https://github.com/rahulnakmol/agentic-ayudha/commit/2c6c6b0) (#1091) | #1015, Feature #889 | #942, #943 |
| A story is Ready only while its Feature's approval covers it | [52f8b96](https://github.com/rahulnakmol/agentic-ayudha/commit/52f8b96) (#1026) | #996, Feature #877 (`_data/merge-refs.tsv`) | #941 |

Commands that the cards will run and that exist today: `aayu plan --approve`, `aayu tell --publish`, `aayu halt`,
`aayu resume`, `aayu decide --waiver`, `aayu ready <f> --part`, and the mission verbs (help output of a build of
origin/dev).

## In review

No open PR refs a Story of #975. Five PRs from other work change files these Stories will edit. On 2026-11-07, one
merged and four closed unmerged when the mission of #1170 ended (`_data/closed-prs.tsv`):

| PR | State on 2026-11-07 | Refs | Overlap |
|---|---|---|---|
| [#1265](https://github.com/rahulnakmol/agentic-ayudha/issues/1265) | closed unmerged on 2026-11-07 when the mission ended; branch `story/1253-aayu-inbox-asks-up-to-five-decisions-to` at 306d1a8 kept | Bug #1253 | Makes "one card per Feature" approve every part, then build and start it: it changes the BASELINE, Approve to build, and Start cards that #951, #953, and #942 extend |
| [#1238](https://github.com/rahulnakmol/agentic-ayudha/issues/1238) | merged on proof on 2026-11-07 as [bc78349](https://github.com/rahulnakmol/agentic-ayudha/commit/bc78349) | Story #1178 | A new overnight card in `inbox.go` and `inboxovernight.go` |
| [#1270](https://github.com/rahulnakmol/agentic-ayudha/issues/1270) | draft; closed unmerged on 2026-11-07 when the mission ended; branch `story/1254-two-person-windows-a-day-outside-them-no` at 7364899 kept | #1254 | Person windows; edits `inbox.go`, `inboxday.go`, `ship.go`, `defaults.toml` |
| [#1276](https://github.com/rahulnakmol/agentic-ayudha/issues/1276) | closed unmerged on 2026-11-07 when the mission ended; branch `fix/approve-keeps-gate` at bcafd0f kept | #1267 | `internal/cli/mission.go` |
| [#1262](https://github.com/rahulnakmol/agentic-ayudha/issues/1262) | closed unmerged on 2026-11-07 when the mission ended; branch `story/1252-aayu-ready-dry-run-for-approve-to-build` at ac3769a kept | #1252 | `internal/cli/mission.go` |

(File lists from `gh pr view <n> --json files`.)

## Pending

All 20 Stories are pending. The Feature body says: "Estimate: 20 stories (4 L, 9 M, 7 S); longest chain #937 → #938
→ #941 → #942; about 11.4 h of mission time at this repository's median open-to-merge hours with 5 agents (Decision #974), under `[mission] warn_hours = 24`" (issue #975, Scope). The label counts in `_data/subs.tsv` match: L #938, #941, #943, #949; S #940, #944, #948, #950, #952, #953, #954; M the other nine.

### Work not sliced into Stories, or outside this Feature

| Gap | Blocks | Source |
|---|---|---|
| 13 core Features with no sub-issues: #871, #872, #873, #879, #880, #881, #882, #883, #884, #885, #886, #888, #897 | 15 Stories need one directly; #942, #951, #952, #953, and #955 need one through #937 or #941 | GitHub API; each Story's Links |
| Remaining sliced core work: #874 (2 of 7 not merged), #875 (0 of 5), #876 (0 of 3), #877 (4 of 6 not merged), #889 (14 planned and 11 ready of the 63 open; 65 in all) | #951, #952, #941, #945, #953, #955, #942, #943, #949 | `_data/subs.tsv` |
| Feature #895 (wave 1), 0 of 15 merged | every Story; #937 needs #925; #949 needs #924 and #936 | issue #975, Links; `_data/subs.tsv` |
| `aayu release --override-evals`: "one Tatkal story adds it (not filed yet)" | #949 C11, C12 | #949 Assumptions |
| Intent, spec, and UX baselines of #975; Hanu's review (D4); Approve to build | every Story's Ready | issue #975, Scope |
| Review guidance and Behaviours sections (Features #887, #891) | every Story's Ready | each Story's Assumptions |

No source gives an estimate for the core Features that are not sliced.
