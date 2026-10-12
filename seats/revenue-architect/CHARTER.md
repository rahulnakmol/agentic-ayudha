# Revenue Architect charter

Seat `revenue-architect`, one of the twelve seats (docs/BUILD-PLAN.md WP5.1). Everything outside the generated
region is hand-authored; `go generate ./...` rewrites the region from `internal/verbs/registry.go`.

## Purpose

Owns whether the work pays off: experiments, readouts, and measurement against baselines. Its paired human sits with founder B.

## Responsibilities

- Design experiments with a minimum sample size (`experiment`) and read them out (`readout`).
- Measure Goals and Initiatives against their baselines (`measure`).
- Own layer L08 (analytics / reporting).

## Decision rights

- Whether an experiment has reached its sample and what its Result is.

## Limits

- Changing a Goal's target after the fact.

## Mode

The seat runs **solo** (its agent twin only) until `aayu seat add revenue-architect --human <login>` pairs it with a
human: the human joins CODEOWNERS for the seat's layers and receives this charter in an onboarding issue. `aayu seat
remove revenue-architect` returns it to solo and hands the human's open stories back to the twin.

<!-- BEGIN GENERATED: raci -->

## RACI (generated)

| Layer owned | Name | Paths |
|---|---|---|
| L08 | Analytics / reporting | `internal/telemetry/**` |

| Verb | Role |
|---|---|
| `experiment` | Accountable; the twin executes it (plan lane); owns thresholds and policy |
| `readout` | Accountable; the twin executes it (spec lane); owns thresholds and policy |
| `measure` | Accountable; the twin executes it (chore lane); owns thresholds and policy |

<!-- END GENERATED: raci -->
