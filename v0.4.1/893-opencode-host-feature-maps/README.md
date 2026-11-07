# OpenCode is a host, and it can maintain the feature maps

| Field | Value |
|---|---|
| Issue | [#893](https://github.com/rahulnakmol/agentic-ayudha/issues/893) |
| Milestone | v0.4.1 (Release [#1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105)); first listed in v0.4.0 (Release [#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906), "includes #893") |
| Status label | `status:planned` (also `priority:P2`; no effort or risk label) |
| Depends on | [#871](https://github.com/rahulnakmol/agentic-ayudha/issues/871) (v0.4.0, open, no filed Stories) |
| Needed by | [#911](https://github.com/rahulnakmol/agentic-ayudha/issues/911) (depends-on #893); [#917](https://github.com/rahulnakmol/agentic-ayudha/issues/917) says it mirrors #893 |
| Code checked | `dev` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b), 2026-11-03 |

## Summary

aayu is to install into OpenCode: MCP, skills, the agent marker, loops, and the `[verify] maintainer` value. This is
step S24 of the core design ([#893](https://github.com/rahulnakmol/agentic-ayudha/issues/893), Summary). On `dev`,
OpenCode is not a host: `verbs.Hosts` has no `opencode` entry (`internal/verbs/registry.go:391-397`), and there is no
`internal/hosts/opencode.go` or `hosts/opencode/`. But part of the adjacent work exists: OpenCode is a delegate
receiver that always runs locally, and it can run a mission's local Kanha, both from
[80af4e1a](https://github.com/rahulnakmol/agentic-ayudha/commit/80af4e1a) (#1125, Story #1012). The issue had put a
delegate receiver for OpenCode out of scope. Nothing in #893's Scope is built, and it has no Stories yet.

## Status

| Item | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | all of Scope |
| Bugs | 0 | 0 | 0 | — |

Sources: GitHub sub-issues API `[]` for #893 (2026-11-07); no row in `_data/subs.tsv`; no PR refs 893 in
`_data/merge-refs.tsv` or `_data/open-prs.tsv`; no `specs/893/` on `origin/ledger`.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Specification](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
