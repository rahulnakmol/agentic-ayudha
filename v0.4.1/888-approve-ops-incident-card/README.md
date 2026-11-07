# Approve an ops action from the INCIDENT card

- Issue: [#888](https://github.com/rahulnakmol/agentic-ayudha/issues/888)
- Milestone: v0.4.1 (Release [#1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105))
- Status label: `status:planned`, `priority:P2` ([#888](https://github.com/rahulnakmol/agentic-ayudha/issues/888))
- Parent: Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870) (v0.4.0)
- Core design step: S19 ([#888](https://github.com/rahulnakmol/agentic-ayudha/issues/888) Summary)

## Summary

`aayu respond <incident> --approve <run>` would approve a pending ops deployment through GitHub's API, as the person.
It is human-only and a one-way door ([#888](https://github.com/rahulnakmol/agentic-ayudha/issues/888)). Today a
person can approve an ops action only on GitHub's Actions page. On `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b), `aayu respond` proposes a catalog action and
dispatches `aayudha-ops.yml`, then prints the run URL for a reviewer (`internal/cli/respond.go:226-228`). It has no
`--approve` flag and makes no pending-deployments call. The Feature is not sliced. Story
[#947](https://github.com/rahulnakmol/agentic-ayudha/issues/947), under the v0.4.0 inbox Feature #975, holds the
INCIDENT card's terminal design.

## Status

| Item kind | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | whole Feature (2 proposed slices) |
| Bugs | 0 | 0 | 0 | — |

Sources: no sub-issues of #888 on GitHub, no row in `_data/subs.tsv`, no PR in `_data/merge-refs.tsv` or
`_data/open-prs.tsv`, no `specs/888` on the ledger.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Spec](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
