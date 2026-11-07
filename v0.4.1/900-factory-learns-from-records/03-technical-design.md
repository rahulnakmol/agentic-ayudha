# Technical design: #900 the factory learns from its own records

No technical baseline exists (no `specs/900/` on `origin/ledger`). Part 1 is what `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b) has. Part 2 is the planned design from
[#900](https://github.com/rahulnakmol/agentic-ayudha/issues/900) and Decision
[#916](https://github.com/rahulnakmol/agentic-ayudha/issues/916). Part 2 is **planned, not built**.

## Layers touched

| Layer | Paths (Expected paths of #900) |
|---|---|
| L02 | `internal/cli/evolve.go`, `retro.go`, `build.go`, `buildnext.go`, `delegate.go`, `streak.go`, `fixlayer.go`, `models.go`, `inspect_app.go` |
| L03 | `loops/evolve.toml` |
| L05 | `internal/models/**` |
| L07 | `internal/ledger/**`, `internal/config/defaults.toml` |
| unknown | `internal/delivery/**`: this package does not exist on 884d81b. Its layer would be L03 by default (`internal/verbs/registry.go`, `LayerTable` comment: "L03 is the default"). |

## Part 1: what exists on dev

| Building block | Where | What it does |
|---|---|---|
| evolve | `internal/cli/evolve.go` (`runEvolve`, `listEvolve`, `EvolveMinFeedback = 2`, `EvolveCooldownDays = 30`, `EvolveBranchPrefix`, `EvolveLabel`) | Accepted retro Feedback about a skill becomes a failing eval case, then a skill diff, as an agent-authored PR. It never merges. |
| Eval-first gate | `internal/evals/evolve.go` | The gate that evolve and CI run. #916 says it moves to the repository's own skill folders. |
| evolve loop | `loops/evolve.toml` (`cron = "0 10 2 * *"`, `command = ["evolve"]`) | Monthly listing. |
| retro | `internal/cli/retro.go` (files Feedback, line 34: "becomes a Feedback issue on this repository (never a PR)"); `aayu retro --classify --layer codebase\|check\|prose\|review` | Files findings as Feedback and classifies them by the layer that can fix them. |
| Recent retro fixes | [2bd966e4](https://github.com/rahulnakmol/agentic-ayudha/commit/2bd966e4) (#1229), [b83e1879](https://github.com/rahulnakmol/agentic-ayudha/commit/b83e1879) (#1233), [fb412a5a](https://github.com/rahulnakmol/agentic-ayudha/commit/fb412a5a) (#1271), [884d81b2](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b2) (#1284), all Refs #1170 | The retro skill states only what its record shows. |
| Blocked builds | `internal/cli/build.go` (`blockBuild`, `markBlocked`, lines 1192-1214) | Free-text reason, page, label `status:blocked`. |
| Mission asks | `internal/cli/missionask.go` (Story #1001) | A mission's ask: one comment on the Feature and one inbox card. A model for FR2 until #880's `aayu ask` exists. Inferred. |
| Review finding class | `internal/inspect/inspect.go` (`Finding.Class`, `Finding.Instances`) | The class of a review finding (Decision #544, per the code comment). |
| Founder minutes measure | `internal/measure/founder.go` (`Founder`, `FounderMetric`), [2e62c537](https://github.com/rahulnakmol/agentic-ayudha/commit/2e62c537) (#1016) | The first metric aayu reads from its own records. |
| Ledger records | `origin/ledger`: `build/<n>/state.json` (36 files), `delegate/`, `inspect/approved.jsonl`, `inspect/dismissed.jsonl`, `retro/<date>/`, `autonomy/audit/` | Records a class counter could read. No per-PR merge record yet (that is #897). |
| Lane defaults | `internal/verbs/registry.go` (`Lanes`, `builderDefaults`) | Constants, chosen by the comments' reasons. |

## Part 2: planned design (planned, not built)

1. **Run outcome.** Each build, delegated build, and loop run writes one outcome: success, flawed, blocked, or
   manual (#900 Scope). Inferred home: the build state on the ledger (`build/<n>/state.json`) or the new
   `internal/delivery/**` of the Expected paths.
2. **Blocked opens an ask.** Uses `aayu ask` from #880 (`asks/<item>/<id>.json`, #880 Scope). #880 is not built.
3. **Three-times rule.** A deterministic counter over findings and failure classes files Feedback at the third
   occurrence, with no model (#900 Scope). Inputs: `inspect.Finding.Class` and the run outcomes of step 1.
4. **evolve trigger.** evolve runs on the Feedback of step 3, not only monthly, and keeps failing-eval-first (#900
   Scope). `loops/evolve.toml` and `EvolveMinFeedback` / `EvolveCooldownDays` change.
5. **Shipped skills refused (#916).** evolve refuses `skills/` here and lock-listed aayu skills in a product
   repository; `[evolve] cooldown_days`, default 7, for a repository's own skills; the eval-first gate moves to those
   skill folders (#916, Code).
6. **Routing evidence (last).** From #897's merge records (one JSON line per merged PR: host, model, effort, tier,
   rounds, fix rounds, tokens or cost), after 4 weeks, compute fix rounds, rework, and cost by lane and model, and show
   them on the Build board (#900 Scope).

## Config keys

- `[evolve] cooldown_days`, default 7 (#916). New. A new key in a rendered section needs a row in
  `internal/config/gen` (`AGENTS.md`, Generated files).

## Decisions

- [#916](https://github.com/rahulnakmol/agentic-ayudha/issues/916), done: evolve changes only a repository's own skills.
- D5 and D10 (issue text). Not in this repository. Unverified.
