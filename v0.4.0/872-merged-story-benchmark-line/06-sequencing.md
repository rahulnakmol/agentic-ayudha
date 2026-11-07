# Sequencing: one benchmark line per merged story

The slices are proposed, not filed (see [05-implementation-plan.md](05-implementation-plan.md)).

## Before any lane starts

1. #871 merges its source (host, session), because #872 depends on it (#872 Links). At least #871's P1 and P3.
2. A person approves #872's baselines and runs Approve to build (`aayu ready 872`). None exists today.

## Waves

Four slices; P2 and P3 are independent once P1 merges. So at most two lanes run at once. The record schema is one
contract, so it goes first and alone.

### Wave 1

| Lane | Slice | Files |
|---|---|---|
| 1 | P1 the delivery record | `internal/delivery/**` (new), `internal/verbs/registry.go` (layer glob), generated files from `go generate ./...` |

### Wave 2 (after P1)

| Lane | Slice | Files |
|---|---|---|
| 1 | P2 `ship` appends the story line | `internal/cli/ship.go`, `internal/cli/build.go`, `internal/cli/ship_test.go`, `internal/cli/build_test.go` |
| 2 | P3 `release` appends the release line | `internal/cli/release.go`, `internal/cli/release_test.go` |

### Wave 3 (after P2)

| Lane | Slice | Files |
|---|---|---|
| 1 | P4 `audit` and `retro` read the lines | `internal/cli/audit.go`, `internal/cli/retro.go`, `internal/audit/**`, `internal/cli/audit_test.go` |

## Hot files

- `internal/verbs/registry.go` and its generated outputs (`docs/README.md`, `LAYERS.md`, `docs/RACI.md`,
  `skills/*/SKILL.md` frontmatter, `internal/mcp/schemas/*.json`; AGENTS.md, "Generated files"). P1 must merge alone
  among lanes that edit the registry; #880 and #881 edit it too.
- `internal/cli/build.go`: shared with #871, #880, and #882.
- `internal/cli/ship.go`: shared with #891 (its Expected paths).

## Person steps and protected paths

- P2 edits `internal/cli/ship.go`, which is in `[autonomy] always_person` (`.aayudha/config.toml:77`). Its PR needs a
  person to ship: ship comments "Ready to ship" and the PR waits in `aayu inbox` (AGENTS.md).
- P1's generated `skills/*/SKILL.md` frontmatter change, if any, is under `/skills/` in `.github/CODEOWNERS`. Inferred:
  adding a layer glob may not change any SKILL.md; check `go generate` output.
- A config key, if one is added, is under `internal/config/` (always_person) and CODEOWNERS.
