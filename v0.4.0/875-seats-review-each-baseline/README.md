# Feature #875: Seats review each baseline in a fresh session

| | |
|---|---|
| Issue | [#875](https://github.com/rahulnakmol/agentic-ayudha/issues/875) |
| Milestone | v0.4.0 (Release [#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906)) |
| Status label | `status:planned` |
| Core design step | S6 ([#875](https://github.com/rahulnakmol/agentic-ayudha/issues/875) Summary) |
| Depends on | [#874](https://github.com/rahulnakmol/agentic-ayudha/issues/874) ([#875](https://github.com/rahulnakmol/agentic-ayudha/issues/875) Links) |

## Summary

A new verb, `aayu critique`, records one structured review per seat of each approved baseline version, and a
host-scheduled loop starts one fresh session per reviewing seat
([#875](https://github.com/rahulnakmol/agentic-ayudha/issues/875) Summary). Nothing of it is built: `dev` has no
`internal/critique`, no `internal/cli/critique.go`, no `skills/critique`, and no `loops/baseline-reviews.toml`. A
few parts that it reads exist already from Features #874 and #877: the review path and finding ID
(`ready.ReviewPath`, `ready.Finding` in `internal/ready/drift.go`), the `[critique] review_wait_hours` key, and the
`Env.CritiqueState` hook that Approve to build reads (`internal/cli/env.go`). The founder approved intent, spec,
and tech v1 on 2026-11-02 (`specs/875/baselines/*-v1.json`). There is no `specs/875/approved.json`, so Approve to
build waits for a person, and no Story of this Feature can be made Ready until it is given (`storyCovered` in
`internal/cli/readycover.go`).

## Status

| State | Stories | Bugs |
|---|---|---|
| Delivered (merged into `dev`) | 0 | 0 |
| In review (open PR) | 0 (`_data/open-prs.tsv` has none) | 0 |
| Pending (not started) | 5 ([#919](https://github.com/rahulnakmol/agentic-ayudha/issues/919), [#920](https://github.com/rahulnakmol/agentic-ayudha/issues/920), [#921](https://github.com/rahulnakmol/agentic-ayudha/issues/921), [#922](https://github.com/rahulnakmol/agentic-ayudha/issues/922), [#923](https://github.com/rahulnakmol/agentic-ayudha/issues/923)) | 0 |
| Not sliced | 1 gap named by a source: Copilot review sessions wait for Feature [#917](https://github.com/rahulnakmol/agentic-ayudha/issues/917) (v0.4.1), with no Story here ([#921](https://github.com/rahulnakmol/agentic-ayudha/issues/921) Assumptions) | none |

Source: `_data/subs.tsv` rows for 875, `_data/merge-refs.tsv` (no merge refs these issues), `_data/open-prs.tsv`.

## Files

1. [01-goal-and-intent.md](01-goal-and-intent.md): goal, customer outcome, problem, measures, out of scope.
2. [02-spec.md](02-spec.md): functional and non-functional requirements, traced to Stories.
3. [03-technical-design.md](03-technical-design.md): layers, packages, ledger records, config keys, decisions.
4. [04-ux-design.md](04-ux-design.md): commands, messages, and the agent's review session.
5. [05-implementation-plan.md](05-implementation-plan.md): every Story with its evidence.
6. [06-sequencing.md](06-sequencing.md): how to build it in parallel lanes.
