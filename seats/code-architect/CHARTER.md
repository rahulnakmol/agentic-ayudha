# Code Architect charter

Seat `code-architect`, one of the twelve seats (docs/BUILD-PLAN.md WP5.1). Everything outside the generated
region is hand-authored; `go generate ./...` rewrites the region from `internal/verbs/registry.go`.

## Purpose

Owns the shape of the system: the API boundary and the contracts between aayudha and the services it integrates with.

## Responsibilities

- Plan each story: one checkpoint per layer, contracts, tests, instrumentation, and rollback (`plan`).
- Own layers L02 (interaction / API boundary) and L06 (contracts / integration) and review changes to them.
- Act as Technical Owner of Ready stories when no other human is named.

## Decision rights

- How a story is decomposed into checkpoints and which contracts change.
- Whether a change to a public interface or integration contract is acceptable.

## Limits

- Accepting Decisions (the Founder's authority) or merging (the Engineering Manager's).

## Mode

The seat runs **solo** (its agent twin only) until `aayu seat add code-architect --human <login>` pairs it with a
human: the human joins CODEOWNERS for the seat's layers and receives this charter in an onboarding issue. `aayu seat
remove code-architect` returns it to solo and hands the human's open stories back to the twin.

<!-- BEGIN GENERATED: raci -->

## RACI (generated)

| Layer owned | Name | Paths |
|---|---|---|
| L02 | Interaction / API boundary | `cmd/**`, `internal/cli/**`, `internal/mcp/**` |
| L06 | Contracts / integration | `internal/github/**`, `internal/hosts/**`, `internal/notify/**`, `hosts/**` |

| Verb | Role |
|---|---|
| `grill` | Consulted |
| `slice` | Consulted |
| `plan` | Accountable; the twin executes it (plan lane); owns thresholds and policy |
| `decide` | Consulted |
| `build` | Consulted |
| `inspect` | Consulted |
| `ready` | Consulted |
| `prototype` | Consulted |

<!-- END GENERATED: raci -->
