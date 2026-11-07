# GitHub Copilot is a first-class host, on this machine and in the cloud, with Auto by default

| Field | Value |
|---|---|
| Issue | [#917](https://github.com/rahulnakmol/agentic-ayudha/issues/917) |
| Milestone | v0.4.1 (Release [#1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105)) |
| Status label | `status:planned` (also `priority:P1`, `effort:L`, `risk:medium`, `horizon:now`) |
| Depends on | [#871](https://github.com/rahulnakmol/agentic-ayudha/issues/871) (open, `status:planned`, v0.4.0) |
| Code checked | `dev` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b), 2026-11-03 |

## Summary

This Feature makes GitHub Copilot a full aayu host: a delegate receiver on this machine (`copilot -p`) and in the cloud
(the Copilot tasks API), with Copilot's Auto model tiers as the lane defaults
([#917](https://github.com/rahulnakmol/agentic-ayudha/issues/917), Summary and Scope C1 to C4). Today Copilot is an
installed host (skills, MCP, hooks; `internal/hosts/copilot.go`, `CopilotProven`) and it can run a mission's local
Kanha (`internal/delegate/local.go`, `LocalKanha`, from
[80af4e1a](https://github.com/rahulnakmol/agentic-ayudha/commit/80af4e1a)). It is not a story receiver:
`aayu build --delegate copilot` fails with `want one of codex, cursor, claude, amp, opencode, auto` (run on 884d81b,
2026-11-07). Nothing of this Feature is built, and it has no Stories yet.

## Status

| Item | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | all of Scope C1 to C4 |
| Bugs | 0 | 0 | 0 | — |

Sources: the GitHub sub-issues API returns no sub-issues for #917 (2026-11-07); `_data/subs.tsv` has no row for 917;
`_data/merge-refs.tsv` and `_data/open-prs.tsv` have no PR that refs 917; the ledger has no `specs/917/` path
(`git ls-tree origin/ledger specs/917` is empty).

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Specification](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
