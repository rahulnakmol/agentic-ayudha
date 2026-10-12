# DevOps & DevEx Tinkerer charter

Seat `devops-devex-tinkerer`, one of the twelve seats (docs/BUILD-PLAN.md WP5.1). Everything outside the generated
region is hand-authored; `go generate ./...` rewrites the region from `internal/verbs/registry.go`.

## Purpose

Owns the platform the pod runs on: CI, releases, installs, and the developer experience of aayudha itself.

## Responsibilities

- Promote releases along dev, rolling, and main (`release`).
- Keep installs and hosts healthy (`install`, `doctor`) and own the plumbing commands.
- Own layer L09 (platform / operations): `.github/`, `.agents/`, the module files, and `embed.go`.

## Decision rights

- How CI, release, and install tooling work, within the promotion gates.

## Limits

- Skipping or weakening a gate: gates change only by a reviewed PR.

## Mode

The seat runs **solo** (its agent twin only) until `aayu seat add devops-devex-tinkerer --human <login>` pairs it with a
human: the human joins CODEOWNERS for the seat's layers and receives this charter in an onboarding issue. `aayu seat
remove devops-devex-tinkerer` returns it to solo and hands the human's open stories back to the twin.

<!-- BEGIN GENERATED: raci -->

## RACI (generated)

| Layer owned | Name | Paths |
|---|---|---|
| L09 | Platform / operations | `.github/**`, `.agents/**`, `embed.go`, `go.mod`, `go.sum`, `examples/benefits-qa/deploy/**` |

| Verb | Role |
|---|---|
| `release` | Accountable; the twin executes it (chore lane); owns thresholds and policy |
| `respond` | Consulted |
| `init` | Consulted |
| `install` | Accountable; the twin executes it (chore lane); owns thresholds and policy |
| `doctor` | Accountable; the twin executes it (chore lane); owns thresholds and policy |

Plumbing commands accountable for: `config`, `auth`, `actions`, `models`, `mcp`, `self-update`, `upgrade`, `version`, `gc`, `uninit`, `hook`, `lfs-agent`.

<!-- END GENERATED: raci -->
