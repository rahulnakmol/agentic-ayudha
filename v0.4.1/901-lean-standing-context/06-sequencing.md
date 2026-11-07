# Sequencing: #901 lean standing context

The slices are the proposed ones in [05-implementation-plan.md](05-implementation-plan.md) (proposed, not filed).

## Before any build

1. File the Stories (`aayu slice 901`). A person approves the baselines and gives Approve to build
   ([af81cb48](https://github.com/rahulnakmol/agentic-ayudha/commit/af81cb48), #1018;
   [45a18e74](https://github.com/rahulnakmol/agentic-ayudha/commit/45a18e74), #1024).
2. A person decides which bar an inspect cut must hold, because the `review` group is advisory (`evals/eval.toml`).
3. Agree with #898 who builds the MCP call log (901-B), so it is built once.

## Waves

| Wave | Lane | Slice | Files or packages it edits |
|---|---|---|---|
| 1 | 1 | 901-A measure | `internal/hosts/hook.go`, `internal/inspect/context.go`, `internal/telemetry/**` (a new measure file) |
| 1 | 2 | 901-B MCP or CLI in the call log | `internal/cli/telemetry.go`, `internal/cli/mcp.go` |
| 2 | 1 | 901-C index variant in the eval harness | `internal/evals/**`, `evals/eval.toml`, `.github/workflows/evals.yml` |
| 3 | 1 | 901-D cut `skills/build` | `skills/build/SKILL.md`, `evals/cases/build/` |
| 3 | 2 | 901-D cut `skills/inspect` | `skills/inspect/SKILL.md`, `evals/cases/inspect/` |
| 4 | 1 | 901-E remove unused verbs and loops (after 30 days of data) | `internal/verbs/registry.go`, `loops/`, `evals/cases/`, generated docs |

Notes:

- 901-A and 901-B both may touch `internal/telemetry/**`. Keep 901-A in a new file there, and 901-B in
  `internal/cli/`, so the lanes do not edit the same file. If both must edit `internal/telemetry/telemetry.go`, merge
  901-B first.
- 901-B may already be built under #898 (its Scope item "MCP calls are logged as MCP calls"). If so, drop the lane.
- Wave 3 runs the two skills as separate lanes. Each touches its own skill and case folder.
- Wave 4 waits 30 days after 901-B merges.

## Why fewer than 10 lanes

The Feature has five slices. Most are an experiment on two skills. The widest wave has two lanes. More lanes would
need more skills in scope, and the issue names only the build and inspect paths.

## Shared hot files

- `internal/verbs/registry.go` (901-E): regenerates `docs/README.md`, `LAYERS.md`, `docs/RACI.md`, skill frontmatter,
  and `internal/mcp/schemas/*.json`. Merge alone.
- `evals/eval.toml` (901-C): every eval change touches it. Do not run 901-C beside another Feature's eval-group edit.

## Steps that need a person

| Step | Why | Source |
|---|---|---|
| 901-D | `/skills/` is a protected path. | `.github/CODEOWNERS` |
| 901-C | `.github/workflows/evals.yml` is under `/.github/`, a protected path. | `.github/CODEOWNERS` |
| 901-E | Removing a case needs an accepted `eval-bar` waiver. Removing a verb is a product change. | `evals/README.md`; inferred |
| Any `[telemetry]` config change | `telemetry` is in `always_person_sections`. | `internal/config/defaults.toml:70` |
| The inspect bar | The `review` group is advisory; a person sets what an inspect cut must hold. | `evals/eval.toml` |
