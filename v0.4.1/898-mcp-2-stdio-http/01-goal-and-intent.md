# Goal and intent: #898 MCP 2.0 on stdio and local HTTP

No intent baseline exists (no `specs/898/` on `origin/ledger`, 2026-11-07). The source is
[#898](https://github.com/rahulnakmol/agentic-ayudha/issues/898), checked against `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b) and the Go SDK v1.8.0 in the module cache.

## Goal

"aayu's MCP server supports the 2026-07-28 MCP specification ("MCP 2.0") on stdio and on a local stateless Streamable
HTTP endpoint, so every agent tool and the web command center use one door." (#898, Summary.) The founder agreed it on
2026-10-21 for v0.4.0 (D7, path B, issue text; unverified here).

## Who it serves

- Agent tools that reach aayu over MCP, including "a host with no shell", which "cannot reach aayu" today (#898,
  Problem).
- The web command center, which "would need a second interface" without this (#898, Problem). The command center is
  not on `dev`; the evidence does not show it.

## Problem, checked against the code

| Claim in #898 | What `dev` and the SDK show |
|---|---|
| The server runs on stdio only. | True. `runMCP` runs `s.Run(ctx, &sdk.IOTransport{…})` (`internal/cli/mcp.go:67`). `aayu mcp --help`: "Serve verbs as MCP tools over stdio." |
| It does not pin the protocol versions it supports. | True. `NewServer` passes only `Instructions` and empty `Capabilities` (`internal/mcp/server.go:92-95`). |
| go-sdk v1.8.0 knows protocol 2026-07-28 (`mcp/shared.go:51`). | True. `protocolVersion20260728 = "2026-07-28"` is `latestProtocolVersion` and first in `supportedProtocolVersions` (`go-sdk@v1.8.0/mcp/shared.go:50-64`). |
| The SDK has multi round-trip requests. | The SDK has `mcp/mrtr.go`. aayu does not use it (no reference in `internal/mcp/`). |
| No cache hints, no discover test, no handle for long verbs. | True for aayu. Long verbs block up to `DefaultTimeout = 15 * time.Minute` with progress every 5 s (`internal/mcp/server.go:49-53`). The SDK has a TTL cache for list and read results (`go-sdk@v1.8.0/mcp/cache.go`). |
| No read tools for the inbox, next, or the meter. | `inbox` and `next` are human-identity plumbing (`internal/verbs/registry.go:1358-1361`). The meter does not exist yet (#897). |

## Success measures

From #898 Scope:

1. The supported versions are pinned, 2026-07-28 first, and a discover conformance test passes.
2. List results carry cache hints.
3. A long verb returns a handle, polled with status.
4. Read tools exist for the inbox, next, and the factory meter, and MCP calls are logged as MCP calls.
5. The ask tool works with multi round-trip forms, for two-way questions only.
6. A local stateless Streamable HTTP endpoint answers on 127.0.0.1, with a local token and the required `Mcp-Method`
   and `Mcp-Name` headers.
7. A 12-month deprecation rule applies to tools.

## Out of scope

From #898, Scope Out: a hosted or remote MCP; any one-way approval through MCP (ship, respond approve, decide accept,
behaviour change approval, release, inspect approve or dismiss, plan approve, scope accept, halt and resume, init,
install, self-update, auth); a device code in any elicitation.
