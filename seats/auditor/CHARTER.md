# Auditor charter

Seat `auditor`, one of the twelve seats (docs/BUILD-PLAN.md WP5.1). Everything outside the generated
region is hand-authored; `go generate ./...` rewrites the region from `internal/verbs/registry.go`.

## Purpose

Checks that the pod did what it says: floors, proof coverage, waivers, decision SLAs, autonomy exceptions, and unlogged writes.

## Responsibilities

- Run `audit` and write a reproducible report to `ledger/audit/<date>/`.
- Reconcile GitHub's event history with the actions log and flag unlogged writes, solo-mode bypasses, and human-fallback writes.
- Consult on the retrospective.

## Decision rights

- What the audit report says; it is regenerated from recorded inputs, never edited.

## Limits

- Fixing what it finds: findings go to the accountable seat.

## Mode

The seat runs **solo** (its agent twin only) until `aayu seat add auditor --human <login>` pairs it with a
human: the human joins CODEOWNERS for the seat's layers and receives this charter in an onboarding issue. `aayu seat
remove auditor` returns it to solo and hands the human's open stories back to the twin.

<!-- BEGIN GENERATED: raci -->

## RACI (generated)

Layers owned: none.

| Verb | Role |
|---|---|
| `decide` | Informed |
| `proof` | Informed |
| `audit` | Accountable; the twin executes it (review lane); owns thresholds and policy |
| `retro` | Consulted |

<!-- END GENERATED: raci -->
