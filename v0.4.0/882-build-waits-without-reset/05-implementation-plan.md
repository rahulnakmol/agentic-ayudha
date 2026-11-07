# Implementation plan: a build waits on a person without a reset

## Stories and Bugs

Feature #882 has no Stories or Bugs (`_data/subs.tsv`). The rows below are proposed, not filed. No source estimates effort
or risk.

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| P1 | A build that asks waits: `waiting` and `paused` in its state, its wall clock paused, and no reset after a clarifying answer | Story (proposed, not filed) | not filed | not estimated | not estimated | #880 (an ask and its answer) | `internal/cli/build.go`, `internal/cli/ask.go` | not started |
| P2 | A waiting build keeps its lease to the provisional time plus 4 h, and the stale sweep skips it | Story (proposed, not filed) | not filed | not estimated | not estimated | P1 | `internal/cli/lease.go`, `internal/cli/gc.go`, `internal/cli/buildnext.go` | not started |
| P3 | The wait limit is a config key in `CONSTRAINTS.md` (inferred) | Story (proposed, not filed) | not filed | not estimated | not estimated | P2 | `internal/config/config.go`, `internal/config/defaults.toml`, `internal/config/gen/main.go`, `CONSTRAINTS.md` | not started |

P3 is inferred from the Expected paths only.

## Delivered

Nothing under #882. Related work on `dev` from Feature #889: the mission clock that pauses on a person
([2c6c6b0](https://github.com/rahulnakmol/agentic-ayudha/commit/2c6c6b04), #1091, Story #1015) and `answeredAsk`,
which keeps a story's slot ([7a9f3d7](https://github.com/rahulnakmol/agentic-ayudha/commit/7a9f3d7e), #1097, Story #1008).

## In review

None (`_data/open-prs.tsv`).

## Pending

- P1 to P3.
- Person steps: #882's baselines and Approve to build (none on the ledger).
- The exact meaning of "the provisional time" for a one-way ask, which never applies provisionally (Decision #862):
  no source says how long the lease holds then. Unverified.
