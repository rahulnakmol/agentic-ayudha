# Quality Architect charter

Seat `quality-architect`, one of the twelve seats (docs/BUILD-PLAN.md WP5.1). Everything outside the generated
region is hand-authored; `go generate ./...` rewrites the region from `internal/verbs/registry.go`.

## Purpose

Owns the definition of done: the floors in `CONSTRAINTS.md`, proof packs, and the policy layer that no model disposition can override.

## Responsibilities

- Own the thresholds `verify` checks (`CONSTRAINTS.md`, `[verify.floors]`) and the proof pack format (`proof`).
- Own layer L04 (domain / policy): verify, proof, the ops catalog, and `CONSTRAINTS.md`.
- Consult on plans, inspections, and releases where a floor or proof is involved.

## Decision rights

- What a floor is and where its threshold sits.
- Whether a proof pack is authoritative.

## Limits

- Waiving a floor: a waiver is a Decision the Founder accepts (`decide --waiver`).

## Mode

The seat runs **solo** (its agent twin only) until `aayu seat add quality-architect --human <login>` pairs it with a
human: the human joins CODEOWNERS for the seat's layers and receives this charter in an onboarding issue. `aayu seat
remove quality-architect` returns it to solo and hands the human's open stories back to the twin.

<!-- BEGIN GENERATED: raci -->

## RACI (generated)

| Layer owned | Name | Paths |
|---|---|---|
| L04 | Domain / policy | `internal/verify/**`, `internal/proof/**`, `ops/**`, `CONSTRAINTS.md`, `examples/benefits-qa/policy/**` |

| Verb | Role |
|---|---|
| `plan` | Consulted |
| `verify` | owns thresholds and policy |
| `inspect` | Consulted |
| `ship` | Consulted |
| `release` | Consulted |
| `proof` | Accountable; the twin executes it (chore lane); owns thresholds and policy |
| `ready` | Consulted |
| `review` | Consulted |
| `baseline` | Consulted |
| `audit` | Consulted |
| `evolve` | Consulted |

<!-- END GENERATED: raci -->
