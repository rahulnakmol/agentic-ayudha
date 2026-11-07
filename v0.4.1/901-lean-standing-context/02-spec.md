# Specification: #901 lean standing context

No spec baseline exists (no `specs/901/` on `origin/ledger`), and no Story is filed, so no Completion Goal criteria
exist. Each requirement traces to [#901](https://github.com/rahulnakmol/agentic-ayudha/issues/901) Scope and to a
proposed slice in [05-implementation-plan.md](05-implementation-plan.md) (proposed, not filed).

## Functional requirements

| ID | Requirement | Proposed slice |
|---|---|---|
| FR1 | aayu measures the standing context of each build session and each inspect session. | 901-A |
| FR2 | MCP calls and CLI calls of each verb and loop are logged, so use over 30 days can be counted. "MCP calls are logged as MCP calls" is also a Scope item of #898. | 901-B (with #898) |
| FR3 | An experiment runs the build and inspect eval cases with an index in place of the full skills. | 901-C |
| FR4 | A cut stays only when it holds every eval bar. | 901-D |
| FR5 | Verbs and loops unused for 30 days, by the MCP and CLI call logs, are removed. | 901-E |

## Non-functional requirements

| ID | Kind | Requirement | Source |
|---|---|---|---|
| NFR1 | Quality | No eval bar is lowered. The ratchet step fails a PR that lowers a bar in `evals/eval.toml`, a case's rule, or a rubric item. | #901 Out; `AGENTS.md`, Coverage ratchet; `evals/README.md`, "The bars only rise" |
| NFR2 | Cost | Eval runs stay under each group's `max_usd`: `host-skills` (build cases) 13.0 USD, `review` (inspect cases) 5.0 USD, `skills` 4.0 USD. | `evals/eval.toml` |
| NFR3 | Privacy | Traces go to a local file by default. Sending them elsewhere needs an explicit `[telemetry] otlp_endpoint`. A call log for FR2 keeps this rule. Inferred. | `internal/telemetry/telemetry.go`, package comment |
| NFR4 | Quality | The changed-line coverage floor is 90.0 percent. | `CONSTRAINTS.md` |

## Points the sources do not settle

- "The build and inspect eval groups" (#901 Scope): `evals/eval.toml` has no group named `build` or `inspect`. Build
  cases (`evals/cases/build/`) fall in the `host-skills` group (prefix `build/`), which gates (`evals/eval.toml`). Inspect cases (`evals/cases/inspect/`)
  are the `review` group, which is advisory and runs on dispatch only (`evals/eval.toml`, `review` group:
  `gate = "advisory"`, `events = ["workflow_dispatch"]`). So an inspect cut cannot be held to a gating bar today.
  A person must decide which bar an inspect cut must hold.
- Removing a verb is a product change. The issue says it happens after 30 days unused. It does not say who approves
  the removal. Inferred: a person, because a verb is a contract (`internal/verbs/registry.go`).
- How a session's standing context is measured (bytes, tokens, or both) is not stated.
