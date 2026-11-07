# Sequencing: Feature #895

## Basis

- Pending work: all 15 sub-issues ([05-implementation-plan.md](05-implementation-plan.md)).
- Depends-on links: #926, #927, #930, #933, #934 → #924; #928 → #924, #925, #927; #932 → #925; #936 → #924 and
  Bug #816 (each issue's Links section).
- The Feature body names the critical path "#924 → #927 → #928" and says the wrap story (#980) runs beside it
  (issue #895, Scope).
- The default number of story threads at once is `[build] agents = 5` (`internal/config/defaults.toml`). This plan
  uses up to 10 lanes, as the brief asks; the Feature's own estimate assumes 5 agents (issue #895, Scope).
- The waves and merge orders below are a plan. No source states them except the depends-on links and the critical
  path.

## Why fewer than 10 lanes

14 of the 15 items edit `internal/cli/inboxtui.go` or `internal/cli/inbox.go` (planned paths in
[05-implementation-plan.md](05-implementation-plan.md)). `inboxtui.go` is 2,737 lines and `inbox.go` is 2,231 lines
(`wc -l` at origin/dev). Both hold one key switch, one `View`, one `inboxKinds` map, and one `inboxOrder` list that
most Stories change. More lanes at once would only make longer rebase chains on those files. So each wave has 5 or 6
lanes, and every wave names its merge order for the shared files.

## Before any build: steps that need a person

| Step | Who | Command or place | Source |
|---|---|---|---|
| Draft the intent, spec, and UX design baselines | an agent | `aayu ready 895 --part <part> --request`, for the parts intent, spec, and ux | `aayu ready --help` |
| Approve each baseline | a person, device flow every time | `aayu ready 895 --part <part>`, or the inbox BASELINE card | `aayu ready --help`; `InboxBaseline` (`internal/cli/inbox.go`) |
| Hanu's review of the web and terminal prototype states (D4) | Hanu (UX Researcher & Evolutionist seat); whether a person also signs is not stated | not stated | issue #895, Scope |
| Approve to build | a person | `aayu ready 895`, or the inbox Approve to build card | issue #895, Scope; `InboxApprove` |
| Plan-time review for plans that touch a never layer (L04, L07, L09) | the `[autonomy] approver` | `aayu plan <story> --approve` | `aayu plan --help`; `[autonomy] never` in `defaults.toml` |
| Decide what to do with the branches of the PRs closed unmerged on 2026-11-07 that touch the same files: #1265, #1270, #1282 (#1238 merged as bc78349) | a person | — | [05-implementation-plan.md](05-implementation-plan.md), "In review"; `_data/closed-prs.tsv` |

## Wave 1: the stale-card base, and the work that touches no inbox view

| Lane | Item | Files or packages | Notes |
|---|---|---|---|
| 1 | #924 (Bug, L) | `internal/cli/inboxtui.go`, `inboxwork.go`, `inboxwhy.go`, `inbox.go` (`reread`, `inboxMaterial`, `inboxAct`), `inbox_test.go` | Critical path. The only lane that edits `inboxtui.go` in this wave. |
| 2 | #1122 (Bug, XS) | `internal/cli/inbox.go` (`canWrite`, `postedCounsel`), `inboxready.go` (`readyItem`), `inbox_test.go` or `inboxready_test.go` | Shares `inbox.go` with lane 1, in other functions. |
| 3 | #980 (Story, S) | `internal/ui/ui.go` (`Printer`), `internal/cli/look.go` | No inbox file. |
| 4 | #935 (Story, M) | `.github/CODEOWNERS`, `internal/config/` (`[codeowners]`), a new inbox file for the owner card, one entry in `inboxKinds` (`inbox.go`) | Narrow set and protected path: a person ships it. |
| 5 | Bug #816 (outside this Feature) | `internal/inspect/`, `internal/cli/inspect.go` | Needed by #936 in wave 3. |

Merge order for `inbox.go`: #1122, then #924, then #935.

## Wave 2: cards and reads on the new base (after #924 merges)

| Lane | Item | Files or packages | Notes |
|---|---|---|---|
| 1 | #927 (Story, L) | `inbox.go` (a bypass kind and its items), `inboxready.go`, `internal/cli/ship.go`, `inboxtui.go` (the reason input) | Critical path. `ship.go` is in `always_person`; the body says "a person ships it". |
| 2 | #925 (Bug, L) | `inboxtui.go` (loading, read state, banners), `inbox.go` (cache writes), `internal/config/defaults.toml` and the config structs (`[inbox]` keys) | `defaults.toml` is a protected path and in `always_person_config`: a person ships it. |
| 3 | #926 (Story, M) | `inbox.go` (`human-review` items, one Approve and ship choice), `internal/inspect/inspect.go`, `internal/cli/inspect.go` | Starts after #816 merges, because both edit inspect code. |
| 4 | #929 (Story, M) | `inboxlogin.go`, `inbox.go` (`openURL`), `inboxtui.go` (o and c), `docs/guide/` | |
| 5 | #930 (Story, M) | `inboxtui.go` (`cardView`), `inbox.go` (`inboxCounsel`) | Sets the card layout that #934 and #936 use. |
| 6 | #931 (Story, S) | `inboxtui.go` (title and herdr state), `internal/verbs/registry.go` (`--brief`) and the generated files, `defaults.toml` (`[ui] bell`) | `defaults.toml`: a person ships it. |

Merge order for `inbox.go` and `inboxtui.go`: #927, #925, #926, #930, #929, #931. The critical path goes first. #925
goes second because #928 and #932 need it. #931 goes last because it also regenerates files from the registry.

Merge order for `defaults.toml`: #925, then #931.

## Wave 3: batch, header, leave, keys, and findings (after #925, #927, and #930 merge)

| Lane | Item | Files or packages | Notes |
|---|---|---|---|
| 1 | #932 (Story, S) | `inboxtui.go` (header and footer in `View`) | |
| 2 | #936 (Story, M) | `inboxtui.go` (findings on the card), `inbox.go` (a WAIVER kind), `internal/cli/inspect.go` | Needs #816 and #924. |
| 3 | #933 (Story, M) | `inboxtui.go` (quit summary), `inbox.go` (cache), `internal/verbs/registry.go` (`--today`, a positional item) and the generated files | |
| 4 | #934 (Story, M) | `inboxtui.go` (`helpView`, every key case), a new keymap source and its generator, `inbox.go` (who may act) | Touches every key binding. |
| 5 | #928 (Story, M) | `inboxtui.go` (Ship select, B, the hold), `defaults.toml` (`batch_hold_seconds`), a test that scans inbox files for duration literals | End of the critical path. `defaults.toml`: a person ships it. |

Merge order for `inboxtui.go`: #932, #936, #933, #934, #928. #934 goes late because it rewrites the key bindings
that the others add. #928 goes last because its test fails on any duration literal in an inbox file, so it then
checks the code of every earlier lane.

## Shared hot spots across #895 and #975

These files are edited by Stories of both inbox waves. They force serialized merges across the two Features:

| File | What both waves change | Stories (wave 1 / wave 2) |
|---|---|---|
| `internal/cli/inbox.go` | `inboxKinds`, `inboxOrder`, `items`, `inboxAct`, the cache | #924 to #936, #1122 / every Story of #975 |
| `internal/cli/inboxtui.go` | the key switch (near line 1367), `View`, `cardView`, `helpView`, `m.status` | #924, #925, #928 to #934, #936 / #937 to #941, #943, #944, #947 to #949, #956 |
| `internal/cli/inbox_test.go` | one 2,752-line test file for the inbox | most Stories of both waves |
| `internal/cli/inboxwhy.go`, `inboxwork.go` | outcome words and write stages | #924 / every card that writes |
| `internal/config/defaults.toml` | `[inbox]` keys | #925, #928, #931 / #949 (`loop_rounds`, `loop_hours`) |
| `internal/verbs/registry.go` and its generated files | new flags | #931, #933 / #943 (`aayu mission --release`) |

Rule for both Features: run #895's waves before #975 starts, because #975 depends on #895 (issue #975, Links), and #937 and #949 depend on Stories of #895 (#925, #924, #936).

## Person steps during the build

- Ship by a person: #925, #927, #928, #931, #935 (narrow set or protected paths, as named above). A protected path
  fails `protected-paths` by design. In solo mode only the founder's recorded admin bypass merges it (AGENTS.md,
  "Protected paths").
- Every Story merges through `aayu ship <pr>` (AGENTS.md, "Branches, PRs, commits").
- PR #1282 proposed "only the policy keys wait for a person" (`_data/open-prs.tsv`). It was a draft and closed
  unmerged on 2026-11-07 when the mission ended; branch `feat/1281-auto-merge-on-proof` at 6f2aae7 kept
  (`_data/closed-prs.tsv`). So the narrow set in `.aayudha/config.toml` applies today.
