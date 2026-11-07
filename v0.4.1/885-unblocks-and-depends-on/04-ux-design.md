# #885 UX design

This Feature has no person-facing screen of its own. The lineage on cards is Story
[#963](https://github.com/rahulnakmol/agentic-ayudha/issues/963) under Feature #976 (v0.4.0, planned). This page
describes the agent-facing surface: the pick commands and what they print.

## Today on dev

- `aayu build --help` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b) describes
  `--next string`: "The build-ready loop, with no model: ship lists the loop's own open PRs, oldest first, and changes
  nothing; pick blocks the loop's stale claims, then, under [build] agents, claims the first Ready story or bug (the
  current release's accepted baseline in its order, else priority, then age) and cuts its worktree."
- `aayu mission next <feature>` lists the mission's startable stories. A held story gets a reason such as
  "depends-on #10 is open" or "depends-on #10 was cancelled" (`internal/cli/missionnext.go:312-315`).
- The build loop's pick prints its order line, for example "no accepted baseline for an open Release: priority, then
  age" (`internal/cli/buildnext.go:562`).

## Planned, not built

- `aayu build --next pick` would skip a held story and say why. The nearest wording is the mission's "depends-on #n
  is open" (inferred).
- The order line would name the unblocks key (inferred).
- #963 gives the card rail, for a later Feature: "Story #796 › PR #802 › proof ✓ › ship › v0.4.0".

## Accessibility

No screen, so no accessibility requirement applies here.
