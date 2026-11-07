# Goal and intent: #900 the factory learns from its own records

No intent baseline exists (no `specs/900/` on `origin/ledger`, 2026-11-07). The sources are
[#900](https://github.com/rahulnakmol/agentic-ayudha/issues/900) and Decision
[#916](https://github.com/rahulnakmol/agentic-ayudha/issues/916), checked against `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b).

## Goal

"The factory learns from its own records: every run ends with an outcome, a failure class seen three times reaches
evolve within a week, and model routing reads its evidence from real pull requests." (#900, Summary.) The founder
agreed it on 2026-10-21 for v0.4.0 (D5 and D10, issue text; not in this repository, unverified).

## Who it serves

The pod's own builders and the founder. A failure that repeats costs them each time it happens. The issue says "the
same failure repeats for weeks" (#900, Problem).

## Problem, checked against the code

| Claim in #900 | What `dev` shows |
|---|---|
| "A blocked run is free text in `status:blocked`". | True. `markBlocked` and `blockBuild` move the story to `status:blocked` with a reason string and page the owner (`internal/cli/build.go:1192-1214`). `aayu build --stop <reason>` does the same (`aayu build --help`). |
| "evolve needs two accepted Feedback items and runs monthly (`internal/cli/evolve.go:51`)". | True. `EvolveMinFeedback = 2`, `EvolveCooldownDays = 30` (`internal/cli/evolve.go:54-55`). `loops/evolve.toml` has `cron = "0 10 2 * *"`, the 2nd of each month. |
| "Model lane defaults are chosen by judgement, with no data on fix rounds, rework, or cost by lane and model." | The lane defaults are constants in `internal/verbs/registry.go` (`Lanes`), with comments that give reasons. The evidence does not show a per-lane measure of fix rounds, rework, or cost. |

Decision #916 (closed, `status:done`) adds a problem: evolve today opens PRs against the skills aayu ships
(`internal/cli/evolve.go`, `evolveBody`, quoted in #916). The founder said "Evolve should not modify aayu's skills we
ship" (#916, Evaluation).

## Success measures

From #900 Scope:

1. Each run ends success, flawed, blocked, or manual. A blocked run opens an ask.
2. A finding or failure class seen three times files Feedback, with no model.
3. evolve runs on that trigger and keeps its failing-eval-first rule.
4. After 4 weeks of merge records, fix rounds, rework, and cost by lane and model show on the Build board, and
   routing changes cite them.

From #916: evolve refuses a shipped skill; `[evolve] cooldown_days` (default 7) applies to a repository's own skills.

## Out of scope

"Replay benchmarks; an LLM foreman." (#900, Scope Out.)
