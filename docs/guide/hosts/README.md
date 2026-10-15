# aayudha in your agent

aayudha works inside the agent you already use. Pick yours:

| Agent | How aayudha arrives |
|---|---|
| [Claude Code](claude-code.md) | A plugin from this repository's marketplace, or `aayu install --host claude` |
| [OpenAI Codex](codex.md) | The same plugin, or `aayu install --host codex` |
| [Amp](amp.md) | `aayu install --host amp`: skills, a project plugin, and the MCP server |
| [Cursor](cursor.md) | `aayu install --host cursor`: skills, rules, MCP, and hooks |

Every host gets the same rules: hooks call only `aayu hook <name>`, steps that belong to a person are never offered
to an agent, and write verbs reach the MCP server only when you set `[mcp] expose_writes = true`.

Then [make it yours](make-it-yours.md): themes, plain mode, and shell completions.
