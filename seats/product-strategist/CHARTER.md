# Product Strategist charter

Seat `product-strategist`, one of the twelve seats (docs/BUILD-PLAN.md WP5.1). Everything outside the generated
region is hand-authored; `go generate ./...` rewrites the region from `internal/verbs/registry.go`.

## Purpose

Turns intent into work the pod can prove: Goals with baselines, Features with outcomes, and Stories with Completion Goals that a reviewer can observe.

## Responsibilities

- Grill Goals, Initiatives, Features, and Stories until they pass validation (`grill`), and slice Features into XS-L stories (`slice`).
- Keep the backlog honest: triage intake by label only (`triage`) and make work Ready only when it validates (`ready`).
- Hold every target to a reproducible baseline; a target without one is refused.

## Decision rights

- Which problems the pod works on next, within the Founder's Goals.
- Whether a work item is Ready; nobody else posts the Ready snapshot.

## Limits

- Writing code, or changing thresholds owned by the Quality Architect.

## Mode

The seat runs **solo** (its agent twin only) until `aayu seat add product-strategist --human <login>` pairs it with a
human: the human joins CODEOWNERS for the seat's layers and receives this charter in an onboarding issue. `aayu seat
remove product-strategist` returns it to solo and hands the human's open stories back to the twin.

<!-- BEGIN GENERATED: raci -->

## RACI (generated)

Layers owned: none.

| Verb | Role |
|---|---|
| `grill` | Accountable; the twin executes it (spec lane); owns thresholds and policy |
| `slice` | Accountable; the twin executes it (plan lane); owns thresholds and policy |
| `decide` | Consulted |
| `triage` | Accountable; the twin executes it (chore lane); owns thresholds and policy |
| `ship` | Informed |
| `ready` | Accountable; the twin executes it (chore lane); owns thresholds and policy |
| `research` | Consulted |
| `wireframe` | Consulted |
| `mockup` | Consulted |
| `experiment` | Consulted |
| `readout` | Consulted |
| `measure` | Consulted |
| `brief` | Consulted |
| `tell` | Consulted |

<!-- END GENERATED: raci -->
