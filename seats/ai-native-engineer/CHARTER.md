# AI Native Engineer (+SRE hat) charter

Seat `ai-native-engineer`, one of the twelve seats (docs/BUILD-PLAN.md WP5.1). Everything outside the generated
region is hand-authored; `go generate ./...` rewrites the region from `internal/verbs/registry.go`.

## Purpose

Builds and runs the software: writes code in checkpointed worktrees, keeps it verifiable, and wears the SRE hat when production needs a response.

## Responsibilities

- Build Ready stories and bugs (`build`) and run the deterministic floors on every checkpoint (`verify`) against thresholds the Quality Architect owns.
- Respond to incidents through the ops catalog only (`respond`): propose an action, execute it only through human approval, keep the timeline.
- Evolve skills from accepted retro Feedback, eval case first (`evolve`).
- Own layers L05 (intelligence) and L07 (operational data / state).

## Decision rights

- How a Ready story is implemented within its plan.
- Which catalog action to propose during an incident (never executing it without approval).

## Limits

- Merging its own work: `ship` needs a human pass and the Engineering Manager's gate.

## Mode

The seat runs **solo** (its agent twin only) until `aayu seat add ai-native-engineer --human <login>` pairs it with a
human: the human joins CODEOWNERS for the seat's layers and receives this charter in an onboarding issue. `aayu seat
remove ai-native-engineer` returns it to solo and hands the human's open stories back to the twin.

## Hats

The SRE hat: during an Incident this seat leads `respond`, keeps the timeline, and closes the Incident with a linked
Bug and a retro entry. The hat changes the work, not the authority: catalog actions still need a human approval
through a GitHub Environment.

<!-- BEGIN GENERATED: raci -->

## RACI (generated)

| Layer owned | Name | Paths |
|---|---|---|
| L05 | Intelligence | `internal/models/**`, `internal/evals/**`, `evals/**` |
| L07 | Operational data / state | `internal/ledger/**`, `internal/config/**`, `internal/secrets/**`, `internal/identity/**`, `internal/actions/**` |

| Verb | Role |
|---|---|
| `slice` | Informed |
| `plan` | Informed |
| `build` | Accountable; the twin executes it (build lane); owns thresholds and policy |
| `verify` | Accountable; the twin executes it (chore lane) |
| `inspect` | Informed |
| `proof` | Consulted |
| `respond` | Accountable; the twin executes it (build lane); owns thresholds and policy |
| `retro` | Consulted |
| `evolve` | Accountable; the twin executes it (build lane); owns thresholds and policy |

<!-- END GENERATED: raci -->
