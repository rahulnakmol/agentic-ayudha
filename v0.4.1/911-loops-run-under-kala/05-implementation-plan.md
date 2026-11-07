# Implementation plan: #911 loops run under Kala

## Filed Stories and Bugs

| Number | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| — | No Story or Bug is filed under #911. | — | — | — | — | — | — | GitHub sub-issues API `[]` (2026-11-07); no row in `_data/subs.tsv`; no PR refs 911 in `_data/merge-refs.tsv` or `_data/open-prs.tsv`. |

## Delivered

Nothing of #911 is delivered. These commits on `dev` are building blocks from #889 and other work:

| Commit | What it gives #911 |
|---|---|
| [93255510](https://github.com/rahulnakmol/agentic-ayudha/commit/93255510) (#1092, Story #1004) | `delegate.Handoff.Scope` and `ready.Parent.Scope`, with comments that reserve scope `repo` for Kala. |
| [941f9ae5](https://github.com/rahulnakmol/agentic-ayudha/commit/941f9ae5) (#1095, Story #1005) | The `mission` skill holds Kanha's protocol: the section the `loop` skill is to share. |
| [2114e008](https://github.com/rahulnakmol/agentic-ayudha/commit/2114e008) (#1087, Story #998) | `aayu mission next` lists startable work with no model call (the pattern for `aayu loop --due`). |
| [80af4e1a](https://github.com/rahulnakmol/agentic-ayudha/commit/80af4e1a) (#1125, Story #1012) | A stateless wake and local sessions for Codex, Copilot, and OpenCode, started and resumed by aayu. |
| [e43876ec](https://github.com/rahulnakmol/agentic-ayudha/commit/e43876ec) (#1103, Story #1009) | An hourly safety tick that wakes a coordinator that missed an event. |
| [26537bd6](https://github.com/rahulnakmol/agentic-ayudha/commit/26537bd6) (#1194) | `[hosts] repos`: the person's tool per repository, from the user config. |

## In review

No open PR refs #911 (`_data/open-prs.tsv`).

## Pending

All of Scope is pending and not sliced. The slices are **proposed, not filed**. The Feature is `effort:L`,
`risk:medium`; the issue gives no estimate per slice.

| Slice | Title (proposed, not filed) | Depends on | Layers and paths |
|---|---|---|---|
| 911-A | `aayu loop --due` lists the due loops as `aayudha.work/v1`, from a cursor, with no model | — | L03 `internal/loops/` (new file); L02 `internal/cli/loops.go`; L03 `internal/verbs/registry.go`; generated docs |
| 911-B | `[loops] check_minutes`, default 15 | — | L07 `internal/config/config.go`, `internal/config/defaults.toml`, `internal/config/gen`; `CONSTRAINTS.md` if the section renders |
| 911-C | `loops/active.json` records the owner, the tool, Kala, and one subthread per loop, and reads the old shape | — | L03 `internal/loops/record.go`; `internal/doctor/**` |
| 911-D | `aayu install --loops` creates Kala in the owner's tool and moves the loops on a tool switch | 911-B, 911-C | L02 `internal/cli/installloops.go` |
| 911-E | The `loop` skill shares one protocol section and the work list with the `mission` skill | 911-A | L03 `skills/loop/` (new), `skills/mission/SKILL.md`; `evals/cases/` (inferred) |
| 911-F | Amp adapter: one Kala thread and one subthread per loop | 911-A, 911-C, 911-D | L03 `internal/loops/` (new `amp` file), `loops/amp/` |
| 911-G | Claude Code, Codex, and Cursor adapters, each with subthreads or one resumed session per loop | 911-A, 911-C, 911-D | L03 `internal/loops/` (one new file per host), `loops/claude/`, `loops/codex/`, `loops/cursor/` |
| 911-H | Copilot and OpenCode adapters | #917, #893, 911-A, 911-C | L03 `internal/loops/loops.go` (`Hosts`), new files; `loops/copilot/`, `loops/opencode/` |
| 911-I | Event loops (`triage-opened`) wake the same subthread | 911-F | L03 `internal/loops/loops.go` (`AmpSubscription`) |
| 911-J | The inbox Team view shows a "Loops" row, and halt covers one loop | #895, 911-C | L02 `internal/cli/inbox*.go`; inferred `internal/cli/loops.go` |

### Gaps the sources do not close

- Which hosts hold a parent and subthreads: "to be checked per host" (#911 Scope). Each of 911-F to 911-H starts with
  that check.
- The issue's owner key `[repos."<owner>/<repo>"] tool` differs from the key on `dev`, `[hosts] repos`
  (`internal/config/config.go:74`). A person should confirm which one applies.
- #889 still has planned Stories that touch the same area: #1126 "The local Kanha's machine runs aayu mission wake
  every check_minutes, set up by aayu install --…" (`status:planned`, `_data/subs.tsv`). Its install step overlaps
  911-D.
- The inbox Team view of FR14 comes from #895 (v0.4.0). The evidence does not show it on 884d81b.
