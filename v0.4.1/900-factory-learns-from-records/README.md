# The factory learns from its own records

| Field | Value |
|---|---|
| Issue | [#900](https://github.com/rahulnakmol/agentic-ayudha/issues/900) |
| Milestone | v0.4.1 (Release [#1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105)); first listed in v0.4.0 (Release [#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906), "includes #900") |
| Status label | `status:planned` (also `priority:P2`; no effort or risk label) |
| Depends on | [#897](https://github.com/rahulnakmol/agentic-ayudha/issues/897) (v0.4.1, not built), [#880](https://github.com/rahulnakmol/agentic-ayudha/issues/880) (v0.4.0, open, no filed Stories) |
| Related decision | [#916](https://github.com/rahulnakmol/agentic-ayudha/issues/916) (done): evolve changes only a repository's own skills; "Feature #900 carries it" |
| Code checked | `dev` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b), 2026-11-03 |

## Summary

Every run is to end with an outcome (success, flawed, blocked, or manual), a failure class seen three times is to
reach `aayu evolve` within a week, and model routing is to read its evidence from real pull requests
([#900](https://github.com/rahulnakmol/agentic-ayudha/issues/900), Summary and Scope). Decision #916 adds that evolve
must refuse aayu's shipped skills and evolve only a repository's own skills. On `dev`, evolve needs two accepted
Feedback issues and waits 30 days between changes (`internal/cli/evolve.go:54-55`), its loop runs monthly
(`loops/evolve.toml`), and a blocked build is free text in `status:blocked` (`internal/cli/build.go`, `markBlocked`).
Nothing of this Feature is built, and it has no Stories yet.

## Status

| Item | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | all of Scope, plus Decision #916 |
| Bugs | 0 | 0 | 0 | — |

Sources: GitHub sub-issues API `[]` for #900 (2026-11-07); no row in `_data/subs.tsv`; no PR refs 900 in
`_data/merge-refs.tsv` or `_data/open-prs.tsv`; no `specs/900/` on `origin/ledger`.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Specification](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
