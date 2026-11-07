# UX design: #898 MCP 2.0 on stdio and local HTTP

The surface is agent-facing: MCP tools, prompts, and resources in an agent tool, and later the web command center. The
person sets it up with `aayu install` and config. Nothing of this Feature is built.

## Today (delivered building blocks)

`aayu mcp --help` on [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b):

```text
Serve verbs as MCP tools over stdio.

Usage:
  aayu mcp [flags]

Flags:
  -h, --help            help for mcp
      --skills string   Skills version of the host plugin starting the server; the server refuses to start when this
                        binary carries another (WP3.2b).
```

The server's instructions to the agent: "aayudha verbs as tools. Write verbs are listed only when [mcp] expose_writes =
true; human-only verbs never are. Pass dry_run to plan a write without performing it." (`internal/mcp/server.go:93`.)

Each tool is titled `aayu <verb>` and carries a read-only hint (`internal/mcp/server.go`, `NewServer`). Resources:
`aayudha://config`, `aayudha://status/{goal}`, `aayudha://ledger/{story}`.

`aayu install` registers the server per host (`hosts/README.md`, MCP registration column): `.mcp.json` for Claude Code,
`.cursor/mcp.json`, a block in `.codex/config.toml`, and Amp's settings. Copilot reads the project's `.mcp.json`
once the person trusts the folder (`internal/hosts/copilot.go`, package comment).

## Planned (planned, not built)

| Moment | Who | What happens (from #898) |
|---|---|---|
| Connect | Agent tool | It negotiates 2026-07-28 first. |
| List tools | Agent tool | The list carries a cache hint, so the client can reuse it. |
| Run a long verb | Agent | It gets a handle at once, and polls status. |
| Read the inbox, next, or the meter | Agent or web command center | A read tool returns it. |
| Ask a two-way question | Agent | A multi round-trip form collects the answer. One-way approvals never go through MCP. |
| Connect over HTTP | Local client | It calls `http://127.0.0.1:<port>` with the local token and the `Mcp-Method` and `Mcp-Name` headers. The port and token setup are not stated. |
| A tool is deprecated | Agent tool | It keeps working for 12 months. The deprecation notice text is not stated. |

## Errors

- The SDK refuses an HTTP request with no `Mcp-Method` header: "missing required Mcp-Method header"
  (`go-sdk@v1.8.0/mcp/streamable_headers.go:369`). It checks only when the request's protocol version is 2026-07-28 or
  later (`minVersionForStandardHeaders`, `streamable_headers.go:30`).
- A request without the local token must be refused. The message is not stated.

## Accessibility

No new screen in this Feature. The web command center is another Feature's surface.
