# Sequencing: Feature #975

## Basis

- Pending work: all 20 Stories ([05-implementation-plan.md](05-implementation-plan.md)).
- Depends-on links inside the Feature (each Story's Links): #938, #939, #940, #943, #947, #948, #951, #954 → #937;
  #941 → #938; #942 → #941; #952, #953 → #951; #955 → #953.
- Depends-on links to other Features: see [03-technical-design.md](03-technical-design.md), "Core Features this one
  depends on". Feature #895 comes first (issue #975, Links).
- The Feature body names the longest chain "#937 → #938 → #941 → #942" (issue #975, Scope).
- The default is `[build] agents = 5` (`internal/config/defaults.toml`). This plan uses up to 10 lanes, as the brief
  asks.
- The waves and merge orders below are a plan. No source states them except the depends-on links and the longest
  chain.

## Why fewer than 10 lanes

- The in-Feature chain is four deep (#937 → #938 → #941 → #942), so the last waves hold only 2 or 3 Stories.
- Every Story adds or changes a card kind in `inboxKinds` and `inboxOrder` (`internal/cli/inbox.go`), and 11 of them
  also edit `internal/cli/inboxtui.go` (planned paths in [05-implementation-plan.md](05-implementation-plan.md)).
- Each wave can start a Story only when its core engine has merged. 13 of those engines have no Stories yet.

## Wave 0: before any Story of #975 can start (outside this Feature)

| Step | Who | Source |
|---|---|---|
| Finish Feature #895 (wave 1), at least #924, #925, and #936 | agents, with the person steps in [../895-inbox-ship-review-cards/06-sequencing.md](../895-inbox-ship-review-cards/06-sequencing.md) | issue #975, Links; #937, #949 Links |
| Slice and build the core Features with no Stories: #873 first (six Stories need it), then #885, #871, #880, #881, #882, #872, #886, #883, #884, #879, #888, #897 | Product Strategist (`slice`), then agents | GitHub API `sub_issues_summary.total = 0`; each Story's Links |
| Finish the sliced core work: #874, #875, #876, #877 (including #1029 and #1082), #889 | agents | `_data/subs.tsv` |
| Decide the milestone of the six v0.4.1 core Features (#879, #883, #884, #885, #888, #897) that v0.4.0 Stories need | a person | issue milestones (GitHub API) |
| Decide whether the areas of #937 replace the decision order of #1175 | a person | [03-technical-design.md](03-technical-design.md), "Overlap with Feature #1170" |
| Check the Stories that cite superseded Decisions #913 and #905 against #915 and #910 | Product Strategist, then a person on any change | comments on #913 and #905 |
| Draft the intent, spec, and UX design baselines of #975 | an agent: `aayu ready 975 --part <part> --request` | `aayu ready --help` |
| Approve each baseline; Hanu's prototype review (D4); Approve to build | a person: `aayu ready 975 --part <part>`, then `aayu ready 975` | issue #975, Scope; `aayu ready --help` |
| Decide what to do with the branches of the PRs closed unmerged on 2026-11-07 that touch the same files: #1265, #1270, #1276, #1262 (#1238 merged as bc78349) | a person | [05-implementation-plan.md](05-implementation-plan.md), "In review" |

## Wave 1: areas and the cards that need only the registry

Starts when #873, #885, and #925 have merged, plus each lane's own engine.

| Lane | Story | Extra engine | Files or packages |
|---|---|---|---|
| 1 | #937 areas | — | `inbox.go` (`inboxOrder` to areas), `inboxtui.go` (area strip, Tab, the unblocks line) |
| 2 | #945 PLAN and BLOCKED | #877, #882 | `inbox.go` (two kinds), `build.go`, `delegate.go` |
| 3 | #946 RELEASE and PUBLISH | — | `inbox.go` (two kinds), `release.go`, the tell verb code |
| 4 | #950 FEEDBACK | — | `inbox.go` (one kind) |
| 5 | #944 timeline | #872 | `inboxtui.go` (`cardView` line, L), `inbox.go` |
| 6 | #956 delivery plan | #872, #886 | `inboxtui.go` (p), `inbox.go` |

Merge order for `inbox.go`: #937, #945, #946, #950, #944, #956. #937 goes first because it sets the areas that the
other kinds join. Merge order for `inboxtui.go`: #937, #944, #956.

## Wave 2: the cards that sit in areas (after #937 merges)

| Lane | Story | Extra engine | Files or packages |
|---|---|---|---|
| 1 | #938 ANSWER | #880, #881, #882, #871 | `inbox.go`, `inboxtui.go` (w, t, d on an ask) |
| 2 | #951 FEATURE, BASELINE, drift | #874 | `inboxbaseline.go`, `inbox.go` |
| 3 | #940 Overdue | #883 | `inboxtui.go` (banner), `inbox.go` |
| 4 | #939 snooze and not sure | #884 | `inboxtui.go` (s and ? menus), `inbox.go` |
| 5 | #948 Emergency stop and budget pause | — | `inbox.go`, `inboxtui.go` (command bar), `halt.go`, `internal/models/gateway.go` |
| 6 | #947 INCIDENT | #888 | `inbox.go`, `inboxtui.go` (Now banner) |
| 7 | #954 TRIAGE | #879 | `inbox.go`, `triage.go` |
| 8 | #949 OVERRIDE and LOOPING | #897, #889; Stories #924, #936 | `inbox.go`, `inboxtui.go`, `decide.go`, `release.go`, `defaults.toml` (`[inbox] loop_rounds`, `loop_hours`) |
| 9 | #943 mission view | #889, #871 | `mission.go`, `internal/verbs/registry.go` (`--release`) and the generated files, `inboxtui.go` (mission view, M, its keys) |

Merge order for `inbox.go` and `inboxtui.go`: #938, #951, #940, #939, #948, #947, #954, #949, #943. #938 goes first
because it is on the longest chain. #951 goes second because #952 and #953 need it. #943 goes last because it is the
largest change to `inboxtui.go` and it regenerates files from the registry.

Key conflict to settle in wave 2: #939 C6 binds ? on an ask to "Not sure", while #934 (wave 1) uses ? for the keys
overlay. #934's keymap source must list this conflict with its resolution (#934 C4 asks for that).

## Wave 3: Ready and approval (after #938 and #951 merge)

| Lane | Story | Extra engine | Files or packages |
|---|---|---|---|
| 1 | #941 REFINE and Make Ready | #871, #877 (`--ack`), #880 | `inbox.go`, `inboxtui.go`, a new REFINE file (inferred) |
| 2 | #953 Approve to build with high-risk entries | #877 (#1029) | `inboxapprove.go`, `inbox.go` |
| 3 | #952 REVIEWED | #875, #876 | `inboxbaseline.go`, `inbox.go` |

Merge order for `inbox.go`: #941, #953, #952.

## Wave 4: mission controls and acknowledgments (after #941 and #953 merge)

| Lane | Story | Extra engine | Files or packages |
|---|---|---|---|
| 1 | #942 mission start and controls | #889 | `missionlock.go` (the Start card), `mission.go`, `inbox.go` |
| 2 | #955 ACK | #877 (#1082) | `inboxapprove.go`, `inbox.go` |

Merge order for `inbox.go`: #942, then #955.

## Shared hot spots across #895 and #975

| File | What both waves change | Wave 1 (#895) | Wave 2 (#975) |
|---|---|---|---|
| `internal/cli/inbox.go` | `inboxKinds`, `inboxOrder`, `items`, `inboxAct` | #924 to #936, #1122 | every Story |
| `internal/cli/inboxtui.go` | the key switch (near line 1367), `View`, `cardView`, `helpView`, `m.status` | #924, #925, #928 to #934, #936 | #937 to #941, #943, #944, #947 to #949, #956 |
| `internal/cli/inbox_test.go` | one 2,752-line test file | most Stories | most Stories |
| `internal/cli/inboxwhy.go`, `inboxwork.go` | outcome words and write stages | #924 | every card that writes |
| `internal/cli/inboxbaseline.go`, `inboxapprove.go` | BASELINE and Approve to build cards | — | #951, #952, #953, #955; PR #1265 (closed unmerged on 2026-11-07, branch kept) |
| `internal/cli/mission.go`, `missionlock.go` | mission verbs and the Start card | — | #942, #943; PRs #1276, #1262 (closed unmerged on 2026-11-07, branches kept) |
| `internal/config/defaults.toml` | `[inbox]` keys | #925, #928, #931 | #949 |
| `internal/verbs/registry.go` and its generated files | new flags | #931, #933 | #943 |
| `internal/cli/release.go` | release verb | — | #946, #949 |

Rule: #975's waves start after #895's, because #975 depends on #895 (issue #975, Links), so the two Features never
edit these files in the same wave.

## Person steps during the build

- Ship by a person: #946 ("Posting the stable request as the person changes who may ask for a release: a person
  ships its PR", #946 Assumptions); #949 (`internal/config/defaults.toml` is in `always_person_config` and in
  `.github/CODEOWNERS`).
- Plan-time review (`aayu plan <story> --approve`) for a plan that touches a never layer (L04, L07, L09): #949 (L07,
  inferred from `defaults.toml`).
- Every Story merges through `aayu ship <pr>` (AGENTS.md).
- Unverified: whether #947 changes a workflow that names an environment, which would put it in the narrow set
  (`.aayudha/config.toml` comment on `[autonomy]`). Its body does not say.
