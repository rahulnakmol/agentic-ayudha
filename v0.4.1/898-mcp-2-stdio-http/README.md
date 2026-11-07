# MCP 2.0: the 2026-07-28 spec on stdio and on a local stateless HTTP endpoint

| Field | Value |
|---|---|
| Issue | [#898](https://github.com/rahulnakmol/agentic-ayudha/issues/898) |
| Milestone | v0.4.1 (Release [#1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105)); first listed in v0.4.0 (Release [#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906), "includes #898") |
| Status label | `status:planned` (also `priority:P1`; no effort or risk label) |
| Depends on | none stated (the issue's Links section is empty) |
| Needed by | [#901](https://github.com/rahulnakmol/agentic-ayudha/issues/901) (depends-on #898) |
| Code checked | `dev` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b), 2026-11-03 |

## Summary

aayu's MCP server is to support the 2026-07-28 MCP specification ("MCP 2.0") on stdio and on a local stateless
Streamable HTTP endpoint on 127.0.0.1, so every agent tool and the web command center use one door
([#898](https://github.com/rahulnakmol/agentic-ayudha/issues/898), Summary). On `dev`, `aayu mcp` serves registry
verbs as tools over stdio only (`internal/cli/mcp.go`, `runMCP`; `internal/mcp/server.go`, `NewServer`). The Go SDK in
`go.mod` is `github.com/modelcontextprotocol/go-sdk v1.8.0`, which already lists `2026-07-28` first among its protocol
versions and has a stateless Streamable HTTP handler. aayu uses neither feature yet. Nothing of this Feature is built,
and it has no Stories yet.

## Status

| Item | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | all of Scope |
| Bugs | 0 | 0 | 0 | — |

Sources: GitHub sub-issues API `[]` for #898 (2026-11-07); no row in `_data/subs.tsv`; no PR refs 898 in
`_data/merge-refs.tsv` or `_data/open-prs.tsv`; no `specs/898/` on `origin/ledger`.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Specification](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
