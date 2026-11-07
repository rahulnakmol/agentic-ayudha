# Sequencing: Review guidance

## Before any lane starts (a person)

1. Approve to build: a person runs `aayu ready 887` through GitHub's device flow. The three baselines are approved
   (`specs/887/baselines/*-v1.json`), so this is the only open person step before Ready. It writes
   `specs/887/approved.json` (Story #995, `aayu ready --help` `--agents`).
2. Optional, a person: decide whether the three differences from Decision #864 ([02-spec.md](02-spec.md)) stay. A
   change to a Story body after Approve to build needs an acknowledgment (#877 Summary).

## Waves

Only three Stories exist, and two depend on the first (spec v1 `depends_on`). So at most two lanes run at once.
More lanes would not help: the work is about 2 M and 1 S (spec v1 efforts).

### Wave 1

| Lane | Story | Files or packages |
|---|---|---|
| 1 | [#1221](https://github.com/rahulnakmol/agentic-ayudha/issues/1221) | `internal/workitems/` (new `ParseGuidance`), `internal/cli/ready.go`, `internal/cli/slice.go`, `internal/planning/slice.go` (inferred, where slice refuses a draft today, `:248`), `internal/ready/ready.go` (proposed: return the body at Ready from `ParseSnapshot`) |

### Wave 2 (after #1221 merges)

| Lane | Story | Files or packages |
|---|---|---|
| 1 | [#1222](https://github.com/rahulnakmol/agentic-ayudha/issues/1222) | `internal/delegate/brief.go`, `internal/cli/delegate.go` |
| 2 | [#1223](https://github.com/rahulnakmol/agentic-ayudha/issues/1223) | `internal/inspect/inspect.go`, `internal/inspect/filter.go`, `internal/cli/inspect.go`, `internal/cli/buildreview.go` |

The two lanes edit no common file if #1221 puts the snapshot body or guidance in `internal/ready/ready.go`. If it
does not, both lanes would edit `ready.go`; then merge #1222 first, because it is the smaller (S).

## Hot files

- `internal/cli/inspect.go` and `internal/inspect/inspect.go`: other planned work may edit inspect too (inferred
  from the title of Feature #899, "A review finding blocks a merge only with proof", milestone v0.4.1). The
  evidence does not show an open PR on these files now (`_data/open-prs.tsv`). Proposed slices of other v0.4.0
  Features edit them too: #873's P4c (an inspect page names its kind) and #891's P3 (the BEHAVIOUR CHANGE hold). #887
  is sliced and its baselines are approved, so #1223 can merge first.
- No Story changes `internal/verbs/registry.go`, so no generated docs change (AGENTS.md, "Generated files").

## Person steps and protected paths

- No planned path is in `.github/CODEOWNERS` or in `[autonomy] always_person` (`.aayudha/config.toml:75-101`). So
  each Story PR can merge on proof through `aayu ship` (AGENTS.md, "Branches, PRs, commits").
- If a lane changes `skills/slice/` to make slice write the block, that path is in CODEOWNERS (`/skills/`), and the
  PR fails `protected-paths` by design.
