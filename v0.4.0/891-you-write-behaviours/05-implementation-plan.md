# Implementation plan: you write behaviours

## Stories and Bugs

Feature #891 has no Stories or Bugs (`_data/subs.tsv`). The rows below are proposed, not filed. Each comes from one part of
the #891 Scope, as Decision #867 option 1 details it. No source estimates effort or risk.

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| P1 | Each map check carries `says`, `rule`, and `source`, and a derived status | Story (proposed, not filed) | not filed | not estimated | not estimated | none | L04: `internal/verify/featuremap.go`, `internal/verify/behaviour.go` | not started |
| P2 | The founder approves or refuses a behaviour with `aayu decide --behaviour`, recorded in `behaviours/approvals.jsonl` | Story (proposed, not filed) | not filed | not estimated | not estimated | P1, #877 (inferred) | L02, L04: `internal/cli/decide.go`, `internal/verify/approvals.go` (new, name inferred) | not started |
| P3 | `inspect` holds a PR that changes an approved check, with a BEHAVIOUR CHANGE card in Decide | Story (proposed, not filed) | not filed | not estimated | not estimated | P2, #873 | L02, L03: `internal/inspect/`, `internal/cli/inspect.go`, `internal/needs/**` | not started |
| P4 | `build` stops when its diff changes an approved check, and the builder asks | Story (proposed, not filed) | not filed | not estimated | not estimated | P2, #880 | L02: `internal/cli/build.go` (inferred; not in #891 Expected paths) | not started |
| P5 | `aayu decide --accept` needs `--sets` | Story (proposed, not filed) | not filed | not estimated | not estimated | P1 | L02: `internal/cli/decide.go`, `internal/verbs/registry.go` (the flag) | not started |
| P6 | A `; standing` criterion needs a map check with its source before `ship` merges | Story (proposed, not filed) | not filed | not estimated | not estimated | P1 | L02, L03: `internal/workitems/`, `internal/cli/ship.go` | not started |
| P7 | This repository's own map, starting with the inbox behaviours | Story (proposed, not filed) | not filed | not estimated | not estimated | P1 | L09: `.agents/skills/verify-aayudha/` | not started |
| P8 | ETHOS principle 12, and the gate code joins `[autonomy] always_person` | Story (proposed, not filed) | not filed | not estimated | not estimated | P2, P3 | `ETHOS.md`, `.aayudha/config.toml` | not started |
| P9 | The Behaviours view data | Story (proposed, not filed) | not filed | not estimated | not estimated | P1, P2 | `internal/verify/` (inferred) | not started; the view is not specified |

## Delivered

Nothing under #891. The engine it extends is on `dev` from before v0.3.0: feature maps and the drift check
(`internal/verify/featuremap.go`, WP4.5b), the behaviour floor (`internal/verify/behaviour.go`, WP4.5b.2), the
evidence map (`internal/verify/evidence.go:43`), and the nightly sweep (`loops/verify-sweep.toml`).

## In review

None (`_data/open-prs.tsv`).

## Pending

- P1 to P9.
- Person steps: #891's baselines and Approve to build (none on the ledger). Decision #867 is accepted, so no new
  Decision is needed for option 1.
- ETHOS principle 11: ETHOS.md has ten principles, and #891 asks for principle 12. The evidence does not show where
  principle 11 comes from. Unverified.
- The Behaviours view: no source says what it shows.
