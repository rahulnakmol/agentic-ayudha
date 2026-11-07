# Implementation plan: #898 MCP 2.0 on stdio and local HTTP

## Filed Stories and Bugs

| Number | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| — | No Story or Bug is filed under #898. | — | — | — | — | — | — | GitHub sub-issues API `[]` (2026-11-07); no row in `_data/subs.tsv`; no PR refs 898. |

## Delivered

Nothing of #898 is delivered. The stdio server it extends landed before v0.3.0 in
[6063ae87](https://github.com/rahulnakmol/agentic-ayudha/commit/6063ae87) (#21). No commit in `_data/merge-refs.tsv`
changes `internal/mcp/` for #898.

## In review

No open PR refs #898 (`_data/open-prs.tsv`).

## Pending

All of Scope is pending and not sliced. The slices are **proposed, not filed**. The issue gives no effort and no
estimate.

| Slice | Title (proposed, not filed) | Depends on | Layers and paths |
|---|---|---|---|
| 898-A | Pin the protocol versions, 2026-07-28 first, with a discover conformance test | — | L02 `internal/mcp/server.go`, `internal/mcp/server_test.go` |
| 898-B | List results carry cache hints | 898-A | L02 `internal/mcp/server.go` |
| 898-C | A long verb returns a handle, polled with status | 898-A | L02 `internal/mcp/handles.go` (new), `internal/mcp/server.go` (`toolHandler`) |
| 898-D | Read tools for the inbox (`--list`) and next | — | L03 `internal/verbs/registry.go` (surfaces); generated `internal/mcp/schemas/*.json` |
| 898-E | A read tool for the factory meter | #897 (the meter), 898-D | L03 `internal/verbs/registry.go`; L02 `internal/mcp/` |
| 898-F | MCP calls are logged as MCP calls | — | L02 `internal/cli/telemetry.go`, `internal/cli/mcp.go` |
| 898-G | The ask tool with multi round-trip forms, two-way only | #880 (`aayu ask`), 898-A | L02 `internal/mcp/ask.go` (new), `internal/mcp/server.go` |
| 898-H | A local stateless Streamable HTTP endpoint on 127.0.0.1 with a local token and the required headers | 898-A | L02 `internal/cli/mcp.go`; L07 `internal/config/config.go`, `internal/config/defaults.toml`; `docs/threat-model.md` |
| 898-I | A 12-month deprecation rule for tools | 898-D | L03 `internal/verbs/registry.go`; L02 `internal/mcp/server.go`; `docs/guide/**` |

### Gaps the sources do not close

- How the SDK lets a server pin versions (898-A). Unverified.
- The token and port design of 898-H.
- Which two hosts must ship Tasks (898-C).
- The web command center that #898 serves is not on `dev`, and no Feature for it was checked here.
