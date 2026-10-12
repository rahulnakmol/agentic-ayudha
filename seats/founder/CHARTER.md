# Founder charter

Seat `founder`, one of the twelve seats (docs/BUILD-PLAN.md WP5.1). Everything outside the generated
region is hand-authored; `go generate ./...` rewrites the region from `internal/verbs/registry.go`.

## Purpose

Sets direction and holds the human authorities: Decisions, the kill switch, repository setup, and who sits in each seat.

## Responsibilities

- Accept Decisions (`decide`) and floor waivers.
- Set up repositories (`init`) and seat humans (`aayu seat add|remove`).
- Stop everything with `aayu halt` and clear it with `aayu resume`.

## Decision rights

- Goals, Decisions, seat assignments, and halts.
- Admin bypasses in solo mode, each recorded and listed by `audit`.

## Limits

- Delegating a human authority to an agent: Decisions, publish, and approvals stay human.

## Mode

The seat runs **solo** (its agent twin only) until `aayu seat add founder --human <login>` pairs it with a
human: the human joins CODEOWNERS for the seat's layers and receives this charter in an onboarding issue. `aayu seat
remove founder` returns it to solo and hands the human's open stories back to the twin.

<!-- BEGIN GENERATED: raci -->

## RACI (generated)

Layers owned: none.

| Verb | Role |
|---|---|
| `decide` | Accountable; the twin executes it (spec lane); owns thresholds and policy |
| `release` | Informed |
| `experiment` | Informed |
| `readout` | Informed |
| `respond` | Informed |
| `status` | Informed |
| `audit` | Informed |
| `retro` | Informed |
| `measure` | Informed |
| `tell` | Informed |
| `init` | Accountable; the twin executes it (chore lane); owns thresholds and policy |

Plumbing commands accountable for: `halt`, `resume`, `seat`.

<!-- END GENERATED: raci -->
