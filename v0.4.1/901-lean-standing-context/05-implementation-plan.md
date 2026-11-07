# Implementation plan: #901 lean standing context

## Filed Stories and Bugs

| Number | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| — | No Story or Bug is filed under #901. | — | — | — | — | — | — | GitHub sub-issues API `[]` (2026-11-07); no row in `_data/subs.tsv`; no PR refs 901 in `_data/merge-refs.tsv` or `_data/open-prs.tsv`. |

## Delivered

Nothing of #901 is delivered. The bar ratchet (`internal/evals/barratchet.go`, `internal/ratchet/evalbars.go`) and the
local traces (`internal/cli/telemetry.go`) that #901 builds on were on `dev` before v0.3.0. No commit in
`_data/merge-refs.tsv` changes them for #901.

## In review

No open PR refs #901 (`_data/open-prs.tsv`).

## Pending

All of Scope is pending and not sliced. The slices are **proposed, not filed**. The Feature has no effort or risk
label, and the issue gives no estimate.

| Slice | Title (proposed, not filed) | Depends on | Layers and paths |
|---|---|---|---|
| 901-A | Measure the standing context of each build and inspect session | — | L06 `internal/hosts/hook.go` (inferred); L03 `internal/inspect/context.go`; L08 `internal/telemetry/**` |
| 901-B | Each verb call records whether it came over MCP or the CLI | #898 (same Scope item) | L02 `internal/cli/telemetry.go`, `internal/cli/mcp.go`; L08 `internal/telemetry/**` |
| 901-C | The eval harness runs a case with an index form of a skill beside the full skill | 901-A | L05 `internal/evals/**`, `evals/eval.toml`, `evals/cases/build/`, `evals/cases/inspect/`; L09 `.github/workflows/evals.yml` |
| 901-D | Keep each index cut of the build and inspect skills that holds every bar | 901-C | L03 `skills/build/SKILL.md`, `skills/inspect/SKILL.md`; `ratchets.toml` |
| 901-E | Remove verbs and loops unused for 30 days | 901-B, 30 days of data | L03 `internal/verbs/registry.go`, `loops/`; generated docs; `evals/cases/` |

### Gaps the sources do not close

- No gating eval group covers inspect today (`review` is advisory, `evals/eval.toml`). 901-D for inspect needs a
  person to say which bar it must hold.
- 901-E removes cases with the verbs they test. `evals.CheckBars` fails a PR that "removes a case" (`evals/README.md`).
  So 901-E needs an accepted `eval-bar` waiver for each removed case (`aayu decide --waiver eval-bar <story>`).
- 901-E needs 30 days of call data after 901-B ships. The calendar, not the code, sets its start.
- The Vercel numbers in the Problem section come from a private research note. Unverified here.
