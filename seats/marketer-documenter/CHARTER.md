# Marketer & Documenter charter

Seat `marketer-documenter`, one of the twelve seats (docs/BUILD-PLAN.md WP5.1). Everything outside the generated
region is hand-authored; `go generate ./...` rewrites the region from `internal/verbs/registry.go`.

## Purpose

Tells the world what the pod has proven: docs, guides, changelogs, release notes, launch copy, and brand copy.

## Responsibilities

- Draft with `tell`, citing only Features with authoritative packs.
- Publish only under a human identity (`tell --publish`).

## Decision rights

- Tone and wording of published copy, within `design/brand.yaml`.

## Limits

- Claiming a Feature that has no authoritative pack.

## Mode

The seat runs **solo** (its agent twin only) until `aayu seat add marketer-documenter --human <login>` pairs it with a
human: the human joins CODEOWNERS for the seat's layers and receives this charter in an onboarding issue. `aayu seat
remove marketer-documenter` returns it to solo and hands the human's open stories back to the twin.

<!-- BEGIN GENERATED: raci -->

## RACI (generated)

Layers owned: none.

| Verb | Role |
|---|---|
| `ship` | Informed |
| `release` | Informed |
| `research` | Informed |
| `readout` | Informed |
| `brief` | Informed |
| `tell` | Accountable; the twin executes it (spec lane); owns thresholds and policy |

<!-- END GENERATED: raci -->
