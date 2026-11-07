# UX design: #901 lean standing context

This Feature has no new person-facing command in its issue. Its surface is agent-facing: the text that a build or
inspect session loads ([#901](https://github.com/rahulnakmol/agentic-ayudha/issues/901), Scope).

## Agent-facing surface

| Surface | Today on 884d81b | After #901 (planned, not built) |
|---|---|---|
| Skill text in a host session | Full skill copies, written by `aayu install` (`internal/hosts/install.go`, `skillCopies`). `skills/build/SKILL.md` is 20,149 bytes. | An index form where an experiment shows it holds every eval bar (#901 Scope). |
| inspect prompt | Context section capped at 40,000 bytes (`internal/inspect/context.go:31`). | The same cap, or a smaller one if an experiment holds the bars. Inferred. |
| Verbs offered | Read verbs always, and write verbs only with `[mcp] expose_writes`, become MCP tools (`internal/mcp/server.go`, `Exposed`, `Tools`); `internal/mcp/schemas/` holds 37 generated schemas. | Verbs unused for 30 days are removed (#901 Scope). |
| Loops | 16 loop files in `loops/`. | Loops unused for 30 days are removed (#901 Scope). |

## Person-facing effects

- A person sees fewer verbs and loops once unused ones are removed. The issue names no notice or deprecation step.
  #898 adds "a 12-month deprecation rule for tools" for MCP tools (#898 Scope). A removal under #901 must respect that
  rule once #898 lands. Inferred.
- An eval report shows the cut's result. The evals workflow already prints results per group
  (`.github/workflows/evals.yml`, `evals/results/`). Inferred.

## Accessibility

No new screen. Nothing to state.
