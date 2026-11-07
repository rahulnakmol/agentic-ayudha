# Sequencing: #898 MCP 2.0 on stdio and local HTTP

The slices are the proposed ones in [05-implementation-plan.md](05-implementation-plan.md) (proposed, not filed).

## Before any build

1. File the Stories (`aayu slice 898`). A person approves the baselines and gives Approve to build
   ([af81cb48](https://github.com/rahulnakmol/agentic-ayudha/commit/af81cb48), #1018;
   [45a18e74](https://github.com/rahulnakmol/agentic-ayudha/commit/45a18e74), #1024).
2. A person approves the HTTP endpoint's security design (token, bind address) before 898-H, because it opens a new
   door (#898 Out: no remote MCP).
3. Agree with #901 that 898-F builds the MCP call log once.

## Waves

| Wave | Lane | Slice | Files or packages it edits |
|---|---|---|---|
| 1 | 1 | 898-A version pin and discover test | `internal/mcp/server.go`, `internal/mcp/server_test.go` |
| 1 | 2 | 898-D inbox and next read tools | `internal/verbs/registry.go`, generated schemas and docs |
| 1 | 3 | 898-F MCP call log | `internal/cli/telemetry.go`, `internal/cli/mcp.go` |
| 2 | 1 | 898-B cache hints | `internal/mcp/server.go` |
| 2 | 2 | 898-H local HTTP endpoint | `internal/cli/mcp.go`, `internal/config/config.go`, `internal/config/defaults.toml`, `docs/threat-model.md` |
| 2 | 3 | 898-I deprecation rule | `internal/verbs/registry.go`, `docs/guide/**` |
| 3 | 1 | 898-C handles for long verbs | `internal/mcp/handles.go` (new), `internal/mcp/server.go` |
| 4 | 1 | 898-G ask tool (after #880) | `internal/mcp/ask.go` (new), `internal/mcp/server.go` |
| 4 | 2 | 898-E meter read tool (after #897) | `internal/verbs/registry.go` |

Order notes:

- `internal/mcp/server.go` is edited by A, B, C, and G. They are in waves 1, 2, 3, and 4.
- `internal/verbs/registry.go` is edited by D, I, and E. They are in waves 1, 2, and 4.
- `internal/cli/mcp.go` is edited by F (wave 1) and H (wave 2).

## Why fewer than 10 lanes

Most slices edit `internal/mcp/server.go` or `internal/verbs/registry.go`. The widest wave has three lanes. New files
(`handles.go`, `ask.go`) reduce the overlap, but each still registers in `NewServer`.

## Shared hot files

- `internal/mcp/server.go`: one slice per wave.
- `internal/verbs/registry.go`: it regenerates `docs/README.md`, `LAYERS.md`, `docs/RACI.md`, skill frontmatter, and
  `internal/mcp/schemas/*.json` (`AGENTS.md`). One slice per wave, and run `go generate ./...` after each rebase.

## Steps that need a person

| Step | Why | Source |
|---|---|---|
| 898-H | `internal/config/` is in `always_person`; `internal/config/defaults.toml` is in `always_person_config`; `mcp` is in `always_person_sections`. | `.aayudha/config.toml:75-102`; `internal/config/defaults.toml:70` |
| 898-H security design | A new local network door; the threat model changes. | #898 Scope; `docs/threat-model.md` |
| Any change to `.mcp.json` in this repository | Protected path. | `.github/CODEOWNERS` |
| 898-G | Must exclude every one-way approval in #898 Out; a person checks the list. Inferred. | #898 Out |
