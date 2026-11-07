# Feature #877: Approve to build gates every story

| | |
|---|---|
| Issue | [#877](https://github.com/rahulnakmol/agentic-ayudha/issues/877) |
| Milestone | v0.4.0 (Release [#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906)) |
| Status label | `status:planned` |
| Core design steps | S8 and S9 ([#877](https://github.com/rahulnakmol/agentic-ayudha/issues/877) Summary) |
| Depends on | [#874](https://github.com/rahulnakmol/agentic-ayudha/issues/874) ([#877](https://github.com/rahulnakmol/agentic-ayudha/issues/877) Links) |
| Replaces | [#878](https://github.com/rahulnakmol/agentic-ayudha/issues/878), folded in by Decision [#903](https://github.com/rahulnakmol/agentic-ayudha/issues/903) |

## Summary

`aayu ready <feature>` is Approve to build: a person, and only a person, agrees a Feature's baselines, its budget,
and every story listed then, once (Decision [#903](https://github.com/rahulnakmol/agentic-ayudha/issues/903)). A
Story is Ready only while that approval covers it (Decision
[#861](https://github.com/rahulnakmol/agentic-ayudha/issues/861)). Both rules are on `dev`: Approve to build
([#995](https://github.com/rahulnakmol/agentic-ayudha/issues/995), `approveToBuild` in
`internal/cli/readyapprove.go`) and the coverage rule
([#996](https://github.com/rahulnakmol/agentic-ayudha/issues/996), `storyCovered` in
`internal/cli/readycover.go`, PR [#1026](https://github.com/rahulnakmol/agentic-ayudha/issues/1026)). Not built:
the acknowledgment for a story added or changed later (`--ack`,
[#1082](https://github.com/rahulnakmol/agentic-ayudha/issues/1082)), the GitHub receipt that proves a person wrote
an approval record (threat T23, [#1083](https://github.com/rahulnakmol/agentic-ayudha/issues/1083)), the per-entry
choices for high-risk changes ([#1029](https://github.com/rahulnakmol/agentic-ayudha/issues/1029)), and Bug
[#1086](https://github.com/rahulnakmol/agentic-ayudha/issues/1086): `aayu grill story` still files Stories with
no parent Feature, which can never be made Ready. Intent, spec, and tech v1 are approved (2026-11-02). There is no
`specs/877/approved.json`, so Approve to build of this Feature waits for a person.

Note on the brief: it calls #1026 a Decision. #1026 is the merged PR that built the coverage rule; the rule is
Decision #861, and its scope is Decision #903 (`gh pr view 1026`; [#996](https://github.com/rahulnakmol/agentic-ayudha/issues/996)
Review guidance).

## Status

| State | Stories | Bugs |
|---|---|---|
| Delivered (merged into `dev`) | 2 ([#995](https://github.com/rahulnakmol/agentic-ayudha/issues/995), [#996](https://github.com/rahulnakmol/agentic-ayudha/issues/996)) | 0 |
| In review (open PR) | 0 (`_data/open-prs.tsv` has none) | 0 |
| Pending (not started) | 3 ([#1029](https://github.com/rahulnakmol/agentic-ayudha/issues/1029), [#1082](https://github.com/rahulnakmol/agentic-ayudha/issues/1082), [#1083](https://github.com/rahulnakmol/agentic-ayudha/issues/1083)) | 1 ([#1086](https://github.com/rahulnakmol/agentic-ayudha/issues/1086)) |
| Not sliced | 6 Scope items of #877 with no Story under it (listed in [05-implementation-plan.md](05-implementation-plan.md)) | — |

Source: `_data/subs.tsv` rows for 877, `_data/merge-refs.tsv`, `_data/open-prs.tsv`.

## Files

1. [01-goal-and-intent.md](01-goal-and-intent.md): goal, customer outcome, problem, measures, out of scope.
2. [02-spec.md](02-spec.md): functional and non-functional requirements, traced to Stories.
3. [03-technical-design.md](03-technical-design.md): layers, packages, ledger records, config keys, decisions.
4. [04-ux-design.md](04-ux-design.md): commands, messages, and inbox cards.
5. [05-implementation-plan.md](05-implementation-plan.md): every Story and Bug with its evidence.
6. [06-sequencing.md](06-sequencing.md): how to build the rest in parallel lanes.
