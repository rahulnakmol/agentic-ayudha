# rahulnakmol/agentic-ayudha#397: Fix at the codebase layer (Feedback #330): parallel agents collide on append-only shared files

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `78f7c90c546dbf8499285f66a7aed3c90663ef91f2497537e29c5cec0e6d39a1` |
| Completion Goal hash | `e7dea33f4ec3e700fac2b893f01663a954d91005168a74133bcbdbbf88e3e8d5` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/397/ready.md` |

Completion Goal at Ready:

1. [metric] state: two branches from the same base each add a decision row; expected: both merge without a conflict, the rendered Appendix B lists both rows, and `TestDecisionIDsUnique` fails when two rows share an ID
2. [api-trace] state: a branch adds an Appendix B row to docs/BUILD-PLAN.md by hand, then runs `go generate ./...`; expected: the row moves to its own file under docs/appendix-b/, the output names docs/appendix-b/ and AGENTS.md as the way to add a row, and `TestAdoptHandWrittenRows` proves it

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

Replaces #334, which `aayu retro --classify` filed from Feedback #330 without the fields Ready needs; this is the same Story with them.

Feedback #330 is a correction classified at the codebase layer: The conflicts come from many writers appending to one file; one file per decision row removes the conflict, and rendering the table with go generate makes a duplicate ID a generation error instead of a review catch.

This Story fixes it there, so the mistake becomes impossible or caught rather than warned against in a skill. Its proof is the new type, invariant, or boundary that the Completion Goal names.

### Completion Goal

1. [metric] state: two branches from the same base each add a decision row; expected: both merge without a conflict, the rendered Appendix B lists both rows, and `TestDecisionIDsUnique` fails when two rows share an ID
2. [api-trace] state: a branch adds an Appendix B row to docs/BUILD-PLAN.md by hand, then runs `go generate ./...`; expected: the row moves to its own file under docs/appendix-b/, the output names docs/appendix-b/ and AGENTS.md as the way to add a row, and `TestAdoptHandWrittenRows` proves it

### Measure

Completion Goal criteria observed.

### Threshold

All 2 criteria.

### Scenario

1. Run `go test ./internal/appendixb/...`: `TestParallelRowsMergeCleanly` makes a scratch repository, adds a row on each of two branches from the same base, merges both, and renders Appendix B; `TestDecisionIDsUnique` checks this repository's rows and a fixture with a duplicate ID.
2. On a scratch branch, add a row to the Appendix B of docs/BUILD-PLAN.md by hand, run `go generate ./...`, and read its output and `git status`.

### Environment

A checkout of this repository on a branch from origin/dev, with Go and git; scratch git repositories in the tests' temporary directories. No network, no production data.

### Checkpoints

1. Rows live one file per row under docs/appendix-b/, every existing row migrated, and the uniqueness and parallel-merge tests pass (criteria 1)
2. go generate moves a hand-written row into its file and names the new way; AGENTS.md and CONTRIBUTING.md say how to add a row (criteria 2)

### Assumptions

- The rendered table is printed on demand and not committed, because a committed rendering collides exactly as the hand-written table does.
````
