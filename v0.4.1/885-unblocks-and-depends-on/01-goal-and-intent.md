# #885 Goal and intent

No intent baseline exists for this Feature: `git ls-tree origin/ledger specs/` lists no `specs/885`. This page uses
[#885](https://github.com/rahulnakmol/agentic-ayudha/issues/885) and Decision
[#918](https://github.com/rahulnakmol/agentic-ayudha/issues/918).

## Goal

Quoted from #885: "A lineage read finds each item's parents, children, dependents, release, and who waits on it,
and the pick loop skips a story whose depends-on target has not merged."

## Who it serves and the customer outcome

- The person and the agents that pick work: a story never starts before what it depends on has merged.
- The person reading a card: they see what an item unblocks (inferred from the "unblocks sort key" in #885 Scope;
  the card design is in Story #963 under #976).

## Problem

Quoted from #885: "`depends-on` reaches only the PR body (`internal/cli/build.go:986-1006`), and nothing reads
dependents (G14)."

On `dev` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b) the first half is out of date.
The mission pick now holds a story with an open depends-on target (`heldDeps`, `internal/cli/missionnext.go:478`,
Story #999). The build loop's `pickNext` (`internal/cli/buildnext.go:416`) takes the current release's accepted baseline
in its order, or else sorts by priority, then created time. It checks no depends-on. `storyDependencies` (`internal/cli/build.go:1068`) still writes the depends-on list into the
PR body. The second half holds: no code reads dependents.

## Scope

In, quoted from #885: "`internal/lineage`, cached per gather with ETags; the unblocks sort key; the depends-on rule in
`pickNext`."

Decision #918 (accepted 2026-10-23) says: "The pick: the mission's pick (#889) returns only stories whose depends-on
have merged, in priority, slice order, then Ready time." So the mission pick is #889's, and #885 keeps the loop's
`pickNext` and the lineage read.

## Out of scope

Quoted from #885: "GitHub's issue-dependencies API."

## Success measures

The sources state no success measure for this Feature.
