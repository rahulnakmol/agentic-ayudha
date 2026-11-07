# Technical design: #901 lean standing context

No technical baseline exists (no `specs/901/` on `origin/ledger`). Part 1 is what `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b) has. Part 2 is the planned design from
[#901](https://github.com/rahulnakmol/agentic-ayudha/issues/901). Part 2 is **planned, not built**.

## Layers touched

From the issue's Expected paths and `LAYERS.md`:

| Layer | Paths |
|---|---|
| L02 | `internal/cli/**` |
| L03 | `skills/**`, `seats/**`, `internal/brief/**`, `internal/verbs/registry.go`, `internal/verbs/gen/**`, `internal/seats/**`, `internal/inspect/**`, `internal/skills/**`, `internal/ratchet/**` |
| L05 | `evals/**`, `internal/evals/**` |
| L06 | `internal/hosts/**` |
| L07 | `internal/actions/**`, `internal/ledger/**`, `internal/config/**` |
| L08 | `internal/telemetry/**` |
| L09 | `.github/workflows/evals.yml` |
| (no layer) | `AGENTS.md`, `ratchets.toml` |

## Part 1: what exists on dev

| Building block | Where | Notes |
|---|---|---|
| Eval harness | `internal/evals/` (`evals.go`, `tier1.go`, `session.go`), `go run ./internal/evals/cmd/eval` | Tier 1 runs in `go test` with no model (`evals/README.md`, Tier 1). |
| Eval groups and bars | `evals/eval.toml` (`[[groups]]`: `brief-plain`, `brief`, `personas`, `retro`, `counsel`, `data-ai`, `production`, `review`, `conversations`, `tell`, `host-skills`, `mission`, `ux`, `survey`, `skills`) | No group is named `build` or `inspect`. `host-skills` (prefixes `build/`, `verify/`, `ready/readback-`, `ready/refuse-`) holds the `evals/cases/build/` cases and gates; it runs on dispatch and on pushes to rolling and main. `review` (prefix `inspect/`) is advisory. `skills` holds every other case. |
| Bar ratchet | `internal/evals/barratchet.go` (`evals.CheckBars`), `internal/ratchet/evalbars.go` | Fails a PR that lowers a bar or removes a case (`evals/README.md`, "The bars only rise"). |
| Evals workflow | `.github/workflows/evals.yml` | Runs the model-graded groups. |
| Skill lint | `internal/skills/lint.go` | Checks skill files. |
| Skill install | `internal/hosts/install.go` (`Plan`, `skillCopies`, `SkillDirs`) | Writes full skill copies per host. |
| Session-start hook | `internal/hosts/hook.go` (`Hooks`: `session-start` "Report skill copies that differ from .aayudha/lock.toml.") | The hook a measure could read from. Inferred. |
| inspect context cap | `internal/inspect/context.go:31` (`maxContextBytes = 40000`) | A fixed cap on the context section of inspect's prompt. |
| Traces | `internal/cli/telemetry.go` (`TracePath`, `startTrace`: `aayu.command`, `aayu.host`, `aayu.dry_run`, `aayu.agent`, `aayu.ci`), `internal/telemetry/telemetry.go` | Local file `telemetry/traces.jsonl` in the state dir. No attribute says the call came over MCP. |
| MCP flag | `internal/cli/env.go:83` (`Options.MCP`), set by `agentRunner` in `internal/cli/mcp.go:31` | The flag exists in-process. It is not recorded in the trace. |
| Actions log | `internal/actions/` | The append-only log of writes through aayu on the ledger. It records writes, not reads. Inferred from `origin/ledger:README.md` ("the append-only log of writes made through `aayu`"). |

## Part 2: planned design (planned, not built)

1. **Measure.** Count the standing context of a build session and an inspect session: the skill, charter, and
   `AGENTS.md` text that reaches the model (#901 Scope). Where to measure is not stated. Candidates: the session-start
   hook for host sessions, and inspect's prompt builder for the merge gate. Inferred.
2. **Call log.** Record each verb call with its surface (MCP or CLI), so the 30-day rule has data (#901 Scope). #898
   has the same Scope item. The trace already has `aayu.command` and `aayu.host`; adding `aayu.mcp` from
   `Options.MCP` is the smallest change. Inferred. A trace file is per machine, so a repository-wide count needs a
   shared record. Inferred.
3. **Index experiment.** Write an index form of `skills/build/SKILL.md` and `skills/inspect/SKILL.md`, and run the
   build and inspect eval cases with each form (#901 Scope). The harness runs a case against one skill text today
   (`internal/evals/`); an A/B variant is new. Inferred.
4. **Keep a cut.** A cut lands only when every bar holds (#901 Scope). The ratchet already blocks a lowered bar.
5. **Remove unused.** Verbs and loops with no call in 30 days are removed from `internal/verbs/registry.go` and
   `loops/` (#901 Scope). Removing a verb regenerates `docs/README.md`, `LAYERS.md`, `docs/RACI.md`, skill frontmatter,
   and `internal/mcp/schemas/*.json` (`AGENTS.md`, Generated files).

## Decisions

- D8 (founder, 2026-10-21, issue text). Not in this repository. Unverified.
- Decision [#916](https://github.com/rahulnakmol/agentic-ayudha/issues/916) (done): aayu's shipped skills change only
  with a release, never through `aayu evolve`. A cut to `skills/build` or `skills/inspect` is release work of this
  kind. Inferred.
