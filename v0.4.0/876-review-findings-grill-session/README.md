# Feature #876: Review findings are resolved in a grill session

| | |
|---|---|
| Issue | [#876](https://github.com/rahulnakmol/agentic-ayudha/issues/876) |
| Milestone | v0.4.0 (Release [#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906)) |
| Status label | `status:planned` |
| Core design step | S7 ([#876](https://github.com/rahulnakmol/agentic-ayudha/issues/876) Summary) |
| Depends on | [#875](https://github.com/rahulnakmol/agentic-ayudha/issues/875) ([#876](https://github.com/rahulnakmol/agentic-ayudha/issues/876) Links) |

## Summary

`aayu grill feature <f> --findings` walks the open review findings with the founder, and each resolution is
recorded. A gap counts as fixed only when a later approved baseline names it, or a person dismisses it with a
reason ([#876](https://github.com/rahulnakmol/agentic-ayudha/issues/876) Summary and Scope). Its three Stories call
it "the pilot Feature" ([#1114](https://github.com/rahulnakmol/agentic-ayudha/issues/1114) Summary). Nothing is
built yet. Approve to build already reads a critique state and refuses on open gaps, but the state is a stub that
reports none (`Env.critique` in `internal/cli/readyapprove.go`). The founder approved intent, spec, and tech v1 on
2026-11-02. There is no `specs/876/approved.json`, so Approve to build waits for a person.

## Status

| State | Stories | Bugs |
|---|---|---|
| Delivered (merged into `dev`) | 0 | 0 |
| In review (open PR) | 0 (`_data/open-prs.tsv` has none) | 0 |
| Pending (not started) | 3 ([#1114](https://github.com/rahulnakmol/agentic-ayudha/issues/1114), [#1115](https://github.com/rahulnakmol/agentic-ayudha/issues/1115), [#1116](https://github.com/rahulnakmol/agentic-ayudha/issues/1116)) | 0 |
| Not sliced | 2 gaps named by sources: the outcomes of inputs (`--as taken` and `--as declined`), deferred in #1114 and #1115 Open questions; the `--words` flag of the Feature Scope, which #1115 drops | none |

Source: `_data/subs.tsv` rows for 876, `_data/merge-refs.tsv` (none), `_data/open-prs.tsv` (none).

## Files

1. [01-goal-and-intent.md](01-goal-and-intent.md): goal, customer outcome, problem, measures, out of scope.
2. [02-spec.md](02-spec.md): functional and non-functional requirements, traced to Stories.
3. [03-technical-design.md](03-technical-design.md): layers, packages, ledger records, config keys, decisions.
4. [04-ux-design.md](04-ux-design.md): commands, messages, and the grill session.
5. [05-implementation-plan.md](05-implementation-plan.md): every Story with its evidence.
6. [06-sequencing.md](06-sequencing.md): how to build it in parallel lanes.
