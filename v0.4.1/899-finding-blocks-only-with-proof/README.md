# A review finding blocks a merge only with proof

| Field | Value |
|---|---|
| Issue | [#899](https://github.com/rahulnakmol/agentic-ayudha/issues/899) |
| Milestone | v0.4.1 (Release [#1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105)); first listed in v0.4.0 (Release [#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906), "includes #899") |
| Status label | `status:planned` (also `priority:P2`; no effort or risk label) |
| Depends on | [#897](https://github.com/rahulnakmol/agentic-ayudha/issues/897) (v0.4.1, not built) |
| Related decision | [#910](https://github.com/rahulnakmol/agentic-ayudha/issues/910) (done, 2026-10-23): "a P0 or P1 blocks until its proof attempt" |
| Code checked | `dev` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b), 2026-11-03 |

## Summary

A review finding is to block a merge only when it carries proof: a fix that passed in the sandbox, or a probe that
fails on the pull request. Other findings become follow-ups
([#899](https://github.com/rahulnakmol/agentic-ayudha/issues/899), Summary). Decision #910 sets the rule in detail: a
P0 or P1 blocks until a probe is tried; a failing probe blocks; no probe, or a passing one, files the finding as its
own Bug; a P0 on a path that `.github/CODEOWNERS` protects always blocks. On `dev`, a finding has no probe or patch
field (`internal/inspect/inspect.go`, `Finding`), and inspect has no follow-up path. Nothing of this Feature is built,
and it has no Stories yet.

## Status

| Item | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | all of Scope |
| Bugs | 0 | 0 | 0 | — |

Sources: GitHub sub-issues API `[]` for #899 (2026-11-07); no row in `_data/subs.tsv`; no PR refs 899 in
`_data/merge-refs.tsv` or `_data/open-prs.tsv`; no `specs/899/` on `origin/ledger`.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Specification](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
