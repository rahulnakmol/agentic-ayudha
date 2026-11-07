# Specification: #898 MCP 2.0 on stdio and local HTTP

No spec baseline exists (no `specs/898/` on `origin/ledger`), and no Story is filed, so no Completion Goal criteria
exist. Requirements trace to [#898](https://github.com/rahulnakmol/agentic-ayudha/issues/898) Scope and to a proposed
slice in [05-implementation-plan.md](05-implementation-plan.md) (proposed, not filed).

## Functional requirements

| ID | Requirement | Proposed slice |
|---|---|---|
| FR1 | The server pins its supported protocol versions, with 2026-07-28 first. | 898-A |
| FR2 | A discover conformance test checks the server. | 898-A |
| FR3 | List results carry cache hints. | 898-B |
| FR4 | A long verb returns a handle, polled with status. MCP Tasks replace the handle when the Go SDK and two target hosts ship Tasks. | 898-C |
| FR5 | Read tools exist for the inbox, next, and the factory meter. | 898-D (inbox, next); 898-E (meter, after #897) |
| FR6 | MCP calls are logged as MCP calls. | 898-F |
| FR7 | The ask tool uses multi round-trip forms, for two-way questions only. | 898-G (after #880) |
| FR8 | A local stateless Streamable HTTP endpoint listens on 127.0.0.1, with a local token and the required `Mcp-Method` and `Mcp-Name` headers. | 898-H |
| FR9 | A tool is deprecated for 12 months before it is removed. | 898-I |

## Non-functional requirements

| ID | Kind | Requirement | Source |
|---|---|---|---|
| NFR1 | Security | No one-way approval goes through MCP: ship, respond approve, decide accept, behaviour change approval, release, inspect approve or dismiss, plan approve, scope accept, halt and resume, init, install, self-update, auth. | #898 Out |
| NFR2 | Security | No device code appears in any elicitation. | #898 Out |
| NFR3 | Security | The HTTP endpoint binds to 127.0.0.1 only and needs a local token. No hosted or remote MCP. | #898 Scope and Out |
| NFR4 | Security | Human-only and admin verbs are never MCP tools; write verbs are tools only with `[mcp] expose_writes = true`. | `internal/mcp/server.go` (`Exposed`); `internal/config/config.go:374` |
| NFR5 | Security | Verbs over MCP run as an agent lane (`AAYU_AGENT=1`), so `--yes` and human-only paths are refused. | `internal/cli/mcp.go` (`agentRunner`) |
| NFR6 | Reliability | Stdout carries only protocol frames; diagnostics go to stderr. | `internal/cli/mcp.go` (`runMCP` comment); `internal/mcp/server.go` (`Runner` comment) |
| NFR7 | Compatibility | A tool keeps working for 12 months after it is marked deprecated. | #898 Scope |
| NFR8 | Compatibility | The server refuses to start when the host plugin carries another skills version. | `internal/cli/mcp.go` (`runMCP`, `--skills`) |

## Points the sources do not settle

- How the local token is made, stored, and given to the client.
- Which two target hosts must ship Tasks before FR4 moves to Tasks.
- Whether `inbox` as an MCP read tool conflicts with its rule "Refused for agents and needs a terminal, except --list"
  (`internal/verbs/registry.go:1361`). Inferred: the read tool maps to `--list`.
