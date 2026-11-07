# Implementation plan: Feature #876

## All Stories and Bugs

Labels from `_data/subs.tsv`. No row of `_data/merge-refs.tsv` or `_data/open-prs.tsv` refs these issues.

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| [#1114](https://github.com/rahulnakmol/agentic-ayudha/issues/1114) | An open review gap keeps Approve to build off until a later approved baseline fixes it, and grill lists each finding | story | planned | M | high | none | L02, L03: `internal/workitems/critique`, `internal/cli/grill.go`, one line in `internal/cli/readyapprove.go` or `env.go` | not started |
| [#1115](https://github.com/rahulnakmol/agentic-ayudha/issues/1115) | A person dismisses a review gap with a reason, and Approve to build ratifies it | story | planned | S | high | #1114 | L02, L03, L07: `internal/verbs/registry.go`, `internal/cli/critique.go`, `internal/config/` | not started |
| [#1116](https://github.com/rahulnakmol/agentic-ayudha/issues/1116) | In the grill session, the agent walks each open gap with the founder, and never dismisses one itself | story | planned | S | medium | #1114, #1115 | L03, L05: `skills/grill/SKILL.md`, `evals/cases/grill/`, `evals/eval.toml` | not started |

## Delivered

No Story of #876 is delivered. The Feature has intent, spec, and tech v1 approved by `rahulnakmol` on 2026-11-02
(`specs/876/baselines/*-v1.json`) and `specs/876/design.json`.

Code from other Features that #876 completes is on `dev`: the `Env.CritiqueState` hook, the open-gap and
dismissal lines in `approveRun.prepare`, and `ready.Dismissal`, from
[45a18e7](https://github.com/rahulnakmol/agentic-ayudha/commit/45a18e7) (PR
[#1024](https://github.com/rahulnakmol/agentic-ayudha/issues/1024), Story #995). `--fixes` is from
[118408f](https://github.com/rahulnakmol/agentic-ayudha/commit/118408f) and
[4aac162](https://github.com/rahulnakmol/agentic-ayudha/commit/4aac162) (Story #993).

## In review

None.

## Pending

- All three Stories. None is `status:ready`.
- Approve to build of #876: a person's step; `specs/876/approved.json` does not exist. Until then `aayu ready`
  refuses each Story (`earlyCovered` in `internal/cli/readycover.go`).
- Work not sliced into Stories:
  - `--as taken` and `--as declined` for inputs: deferred (#1114 and #1115 Open questions). The Feature Scope
    lists `--as fixed|dismissed|taken|declined`; only `dismissed` has a Story. `fixed` is reached through
    `--fixes` on a new version instead (#1114 CG 2).
  - `--words` from the Feature Scope: #1115 says the command "takes no --words", because the person types the
    reason.
  - `internal/needs/**` is in the Feature's Expected paths, and #920 CG 10 speaks of "the findings Need (#876)".
    No Story of #876 names a Need. I found no `internal/needs` package on `dev` (`ls internal/needs` fails).
- No source gives an estimate beyond the effort labels (M, S, S).
