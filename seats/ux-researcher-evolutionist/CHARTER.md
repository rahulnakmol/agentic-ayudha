# UX Researcher & Evolutionist charter

Seat `ux-researcher-evolutionist`, one of the twelve seats (docs/BUILD-PLAN.md WP5.1). Everything outside the generated
region is hand-authored; `go generate ./...` rewrites the region from `internal/verbs/registry.go`.

## Purpose

Owns the experience: research, wireframes, mockups, prototypes, and the UI review that decides whether an interface ships.

## Responsibilities

- Research with pinned references, URLs and IDs only (`research`), then `wireframe`, `mockup`, and `prototype`.
- Own `review`: critique UI with a model lane and a deterministic lane, then `polish` and `baseline`.
- Own layer L01 (experience) and the design system in `packages/design-system/`.

## Decision rights

- Whether a UI change is ready to ship from an experience point of view (`review` disposition).

## Limits

- Using images from Mobbin or any reference source in the product (links only).

## Mode

The seat runs **solo** (its agent twin only) until `aayu seat add ux-researcher-evolutionist --human <login>` pairs it with a
human: the human joins CODEOWNERS for the seat's layers and receives this charter in an onboarding issue. `aayu seat
remove ux-researcher-evolutionist` returns it to solo and hands the human's open stories back to the twin.

<!-- BEGIN GENERATED: raci -->

## RACI (generated)

| Layer owned | Name | Paths |
|---|---|---|
| L01 | Experience | `packages/design-system/**` |

| Verb | Role |
|---|---|
| `grill` | Consulted |
| `research` | Accountable; the twin executes it (spec lane); owns thresholds and policy |
| `wireframe` | Accountable; the twin executes it (build lane); owns thresholds and policy |
| `mockup` | Accountable; the twin executes it (build lane); owns thresholds and policy |
| `prototype` | Accountable; the twin executes it (build lane); owns thresholds and policy |
| `review` | Accountable; the twin executes it (ui-review lane); owns thresholds and policy |
| `polish` | Accountable; the twin executes it (build lane); owns thresholds and policy |
| `baseline` | Accountable; the twin executes it (chore lane); owns thresholds and policy |
| `experiment` | Consulted |

<!-- END GENERATED: raci -->
