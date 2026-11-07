# Technical design: #898 MCP 2.0 on stdio and local HTTP

No technical baseline exists (no `specs/898/` on `origin/ledger`). Part 1 is what `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b) and the Go SDK v1.8.0 have. Part 2 is the
planned design from [#898](https://github.com/rahulnakmol/agentic-ayudha/issues/898). Part 2 is **planned, not built**.

## Layers touched

| Layer | Paths (Expected paths of #898) |
|---|---|
| L02 | `internal/mcp/**`, `internal/cli/**` |
| L03 | `internal/verbs/registry.go` |
| L07 | `internal/config/**` |
| (no layer) | `docs/guide/**`, `docs/threat-model.md` |

## Part 1: what exists on dev

| Building block | Where | What it does |
|---|---|---|
| Server | `internal/mcp/server.go` (`NewServer`, `Options`, `Exposed`, `Tools`, `toolSchema`, `Args`, `toolHandler`) | One tool per exposed verb. Added in [6063ae87](https://github.com/rahulnakmol/agentic-ayudha/commit/6063ae87) (#21, WP3.3). |
| Exposure rule | `internal/mcp/server.go` (`Exposed`) | Read verbs always; write verbs only with `[mcp] expose_writes`; human-only and admin verbs never. |
| Schemas | `internal/mcp/schemas/*.json` (37 files) | Generated from `internal/verbs/registry.go` (`AGENTS.md`, Generated files). |
| Prompts | `internal/mcp/server.go` (`addPrompts`, `addPersonaPrompt`, `askVerbs = build, plan, slice, grill`) | grill, ask (a request in words, WP5.8), brief, and seat personas. |
| Resources | `internal/mcp/server.go:493-525` (`ConfigURI` `aayudha://config`, `StatusURI` `aayudha://status/{goal}`, `LedgerURI` `aayudha://ledger/{story}`) | Read through `aayu config show`, `aayu status`, `aayu proof get`. |
| Long calls | `internal/mcp/server.go:49-53` (`DefaultTimeout = 15m`, `DefaultProgress = 5s`) | A call blocks and sends progress notifications. |
| Transport | `internal/cli/mcp.go:53-70` (`runMCP`, `sdk.IOTransport`) | stdio only. |
| Agent lane | `internal/cli/mcp.go:17-36` (`agentRunner`: `AAYU_AGENT=1`, `opts.MCP = true`) | Each tool call runs the verb in-process as an agent. |
| MCP flag | `internal/cli/env.go:83` (`Options.MCP`); read in `internal/cli/auth.go:31` | Not written into the trace (`internal/cli/telemetry.go`, `startTrace` attributes). |
| Config | `internal/config/config.go:374` (`ExposeWrites`, key `[mcp] expose_writes`) | The only `[mcp]` key. |
| SDK versions | `go-sdk@v1.8.0/mcp/shared.go:50-64` | `2026-07-28` first, then `2025-11-25`, `2025-06-18`, `2025-03-26`, `2024-11-05`. |
| SDK HTTP | `go-sdk@v1.8.0/mcp/streamable.go` (`StreamableHTTPHandler`, `StreamableHTTPOptions.Stateless`), `streamable_headers.go` (`Mcp-Method`, `Mcp-Name`) | "In Stateless mode, GET and DELETE requests return 405"; a request missing `Mcp-Method` is refused when its protocol version is 2026-07-28 or later (`validateMcpHeaders`, `minVersionForStandardHeaders`, `streamable_headers.go:30`, `:358-369`). |
| SDK cache | `go-sdk@v1.8.0/mcp/cache.go` (`methodCache`, `GetTTLMs`) | "a per-method TTL cache for list and read results". |
| SDK multi round-trip | `go-sdk@v1.8.0/mcp/mrtr.go` | Present. Not used by aayu. |
| SDK Tasks | — | A search of `go-sdk@v1.8.0/mcp/protocol.go` finds no Tasks type. Consistent with "Tasks when the Go SDK … ships it" (#898 Scope). |

## Part 2: planned design (planned, not built)

1. **Version pin and discover test (FR1, FR2).** Set the server's supported versions with 2026-07-28 first, and add a
   conformance test that runs discovery against `NewServer`. How v1.8.0 lets a server restrict versions is not
   checked. Unverified.
2. **Cache hints (FR3).** Set a TTL on list results. The SDK's results carry `GetTTLMs` (`cache.go`). Inferred.
3. **Handles (FR4).** A long verb returns a handle at once; a status tool polls it. Replace with MCP Tasks once the SDK
   and two hosts have them (#898 Scope).
4. **Read tools (FR5).** `inbox --list`, `next`, and the factory meter of #897, as read-only tools. The registry decides
   surfaces (`verbs.SurfaceMCP`), so the change is in `internal/verbs/registry.go`.
5. **Call log (FR6).** Write `Options.MCP` into the trace or a call record. #901 needs the same data.
6. **Ask tool (FR7).** Multi round-trip forms for two-way questions, using `mrtr.go`. Needs `aayu ask` from #880.
7. **HTTP endpoint (FR8).** A new transport in `runMCP` (or a flag) that serves `StreamableHTTPHandler` with
   `Stateless: true` on 127.0.0.1, checks a local token, and requires the `Mcp-Method` and `Mcp-Name` headers. New
   `[mcp]` keys for the port and token source. Inferred.
8. **Deprecation (FR9).** A deprecation date per tool in the registry, and a test that refuses removal before 12
   months. Inferred.

## Threat model

`docs/threat-model.md` names "MCP exposure" as a control (line 44) and T6, prompt injection that makes an agent write,
ship, publish, or run an ops action (line 61). A new HTTP door needs a row for local-network access and the token.
Inferred; #898 lists `docs/threat-model.md` in Expected paths.

## Decisions

- D7, path B (issue text). Not in this repository. Unverified.
