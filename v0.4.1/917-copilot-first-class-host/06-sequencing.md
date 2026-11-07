# Sequencing: #917 GitHub Copilot as a first-class host

The slices are the proposed ones in [05-implementation-plan.md](05-implementation-plan.md) (proposed, not filed).

## Before any build

These steps need a person:

1. File the Stories (`aayu slice 917`), then write and approve the Feature baselines: intent, spec, and technical
   design (`aayu ready 917 --part <p> --request`). Only a person approves a baseline at the hash they saw
   ([af81cb48](https://github.com/rahulnakmol/agentic-ayudha/commit/af81cb48), #1018).
2. A person gives Approve to build ([45a18e74](https://github.com/rahulnakmol/agentic-ayudha/commit/45a18e74), #1024).
3. Decide the dependency on #871. It is open, with no filed Stories. Either build #871's part first, or record that
   #917 does not need it.
4. A live probe of the Copilot tasks API and of the plan check, on a Business or Enterprise subscription (#917 C4).
   It needs a person's Copilot account. The guide records the last live run as 2026-10-17
   (`docs/guide/hosts/copilot.md`).

## Waves

| Wave | Lane | Slice | Files or packages it edits |
|---|---|---|---|
| 1 | 1 | 917-A Auto tiers | `internal/verbs/registry.go`, `internal/models/resolve.go`, `internal/cli/models.go`, `docs/guide/how-models.md`, generated docs |
| 1 | 2 | 917-C Copilot token only | `internal/hosts/copilot.go`, `internal/models/copilot.go` |
| 1 | 3 | 917-E setup steps and plan check | `internal/hosts/install.go`, `hosts/**` (a new template), `internal/doctor/checks.go`, `.github/workflows/copilot-setup-steps.yml` |
| 2 | 1 | 917-D local receiver | `internal/delegate/state.go`, `internal/delegate/local.go`, `internal/cli/delegate.go`, `internal/verbs/registry.go` (enum only) |
| 2 | 2 | 917-B review on Auto | `internal/models/resolve.go` (`CheckFamilies`), `internal/inspect/**` |
| 3 | 1 | 917-F cloud receiver | `internal/delegate/receivers.go`, `internal/delegate/state.go`, `internal/cli/delegate.go` |
| 4 | 1 | 917-G guide | `docs/guide/hosts/copilot.md`, `docs/guide/hosts/README.md` |

Why the waves are in this order:

- 917-D needs the lane tier from 917-A and the token rule from 917-C (#917 C3).
- 917-B and 917-A both edit `internal/models/resolve.go`, so 917-B waits for wave 2.
- 917-F and 917-D both edit `internal/delegate/state.go` and `internal/cli/delegate.go`, so 917-F waits for wave 3.
- 917-G documents the final commands, so it is last.

## Why fewer than 10 lanes

The Feature has seven slices, and most of them share the delegate files. The widest wave has three lanes. Splitting
further would put two lanes on `internal/delegate/state.go` or `internal/verbs/registry.go` in one wave.

## Shared hot files

- `internal/verbs/registry.go`: 917-A and 917-D. It generates `docs/README.md`, `LAYERS.md`, `docs/RACI.md`,
  `skills/*/SKILL.md` frontmatter, and `internal/mcp/schemas/*.json` (`AGENTS.md`, Generated files). Merge 917-A first;
  917-D rebases and reruns `go generate ./...`.
- `internal/delegate/state.go` and `internal/cli/delegate.go`: 917-D, then 917-F.
- `internal/models/resolve.go`: 917-A, then 917-B.

## Steps that need a person

| Step | Why | Source |
|---|---|---|
| 917-E's `.github/workflows/copilot-setup-steps.yml` | `/.github/` is a protected path in `.github/CODEOWNERS`. A workflow that reads a secret also needs a person. | `.github/CODEOWNERS`; `AGENTS.md`, Branches, PRs, commits |
| A change to `internal/config/defaults.toml` (an Expected path of #917) | It is in `always_person_config` and in `.github/CODEOWNERS`. | `.aayudha/config.toml:102`; `.github/CODEOWNERS` |
| A change to a `[models]` config section | `models` is in `always_person_sections`. | `internal/config/defaults.toml:70` |
| The live probes for 917-E and 917-F | They need a person's Copilot plan. | #917 C4 |
| 917-B's exception to the cross-family rule | It changes a review rule in `AGENTS.md`. Inferred: a person should confirm the rule text. | `AGENTS.md`, Work items |

Note: draft PR [#1282](https://github.com/rahulnakmol/agentic-ayudha/issues/1282) (head 6f2aae7, Story #1281,
Decision #1255) changed `always_person` itself. It closed unmerged on 2026-11-07 when the mission ended; branch `feat/1281-auto-merge-on-proof` at `6f2aae7` kept (`_data/closed-prs.tsv`). If Story #1281 is built again, recheck this table.
