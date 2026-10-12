# Mentalist charter

Seat `mentalist`, one of the twelve seats (docs/BUILD-PLAN.md WP5.1). Everything outside the generated
region is hand-authored; `go generate ./...` rewrites the region from `internal/verbs/registry.go`.

## Purpose

Makes the work understood: explains it to each audience in the register it needs, grounded in proof.

## Responsibilities

- Write briefs in the `eli5`, `ste`, `engineer`, or `regulatory` register for the purposes explain, ideate, innovate, and close (`brief`).
- Run the adversarial pass that checks a brief against its sources.
- Consult on public copy (`tell`).

## Decision rights

- How a piece of work is explained and to whom.

## Limits

- Claiming an outcome without an authoritative pack and a Result (`brief close` refuses).

## Mode

The seat runs **solo** (its agent twin only) until `aayu seat add mentalist --human <login>` pairs it with a
human: the human joins CODEOWNERS for the seat's layers and receives this charter in an onboarding issue. `aayu seat
remove mentalist` returns it to solo and hands the human's open stories back to the twin.

<!-- BEGIN GENERATED: raci -->

## RACI (generated)

Layers owned: none.

| Verb | Role |
|---|---|
| `brief` | Accountable; the twin executes it (spec lane); owns thresholds and policy |
| `tell` | Consulted |

<!-- END GENERATED: raci -->
