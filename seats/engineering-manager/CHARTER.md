# Engineering Manager charter

Seat `engineering-manager`, one of the twelve seats (docs/BUILD-PLAN.md WP5.1). Everything outside the generated
region is hand-authored; `go generate ./...` rewrites the region from `internal/verbs/registry.go`.

## Purpose

Owns flow and the merge decision: work moves from Ready to merged only through a green gate, an adversarial inspection, and a human pass.

## Responsibilities

- Own the `inspect` policy that the review lane executes, and `ship` merges after a green merge gate and a human pass.
- Roll up status, health, risk, and cost (`status`), and run the retrospective (`retro`).
- Own layer L03 (application / workflow), including `skills/`, `seats/`, and the work-item schema.
- Review plans for `[autonomy] never` layers before build starts.

## Decision rights

- Whether a PR ships, and what the inspection policy checks.
- Which findings from `retro` become Feedback issues.

## Limits

- Changing floors in `CONSTRAINTS.md` (the Quality Architect's).

## Mode

The seat runs **solo** (its agent twin only) until `aayu seat add engineering-manager --human <login>` pairs it with a
human: the human joins CODEOWNERS for the seat's layers and receives this charter in an onboarding issue. `aayu seat
remove engineering-manager` returns it to solo and hands the human's open stories back to the twin.

<!-- BEGIN GENERATED: raci -->

## RACI (generated)

| Layer owned | Name | Paths |
|---|---|---|
| L03 | Application / workflow | `internal/verbs/**`, `internal/workitems/**`, `internal/loops/**`, `loops/**`, `work-items/**`, `skills/**`, `seats/**`, `examples/benefits-qa/ask/**` |

| Verb | Role |
|---|---|
| `grill` | Informed |
| `slice` | Consulted |
| `plan` | Consulted |
| `decide` | Informed |
| `triage` | Informed |
| `build` | Informed |
| `verify` | Informed |
| `inspect` | Accountable; the twin executes it (review lane); owns thresholds and policy |
| `ship` | Accountable; the twin executes it (chore lane); owns thresholds and policy |
| `release` | Consulted |
| `ready` | Informed |
| `wireframe` | Informed |
| `mockup` | Informed |
| `prototype` | Informed |
| `review` | Informed |
| `polish` | Informed |
| `baseline` | Informed |
| `respond` | Informed |
| `status` | Accountable; the twin executes it (chore lane); owns thresholds and policy |
| `retro` | Accountable; the twin executes it (spec lane); owns thresholds and policy |
| `evolve` | Informed |
| `init` | Informed |
| `install` | Informed |
| `doctor` | Informed |

<!-- END GENERATED: raci -->
