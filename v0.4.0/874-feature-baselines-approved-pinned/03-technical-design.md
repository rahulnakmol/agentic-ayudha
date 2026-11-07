# Technical design: Feature #874

## Tech baseline

`specs/874/baselines/tech-v1.json` on `ledger`: owner `code-architect`, approved by `rahulnakmol` at
2026-10-30T19:10:48Z, hash `4d3ec93eeb65…`. It pins `specs/874/design.json` by SHA-256 `1ef0e5e30343…`. That design
record (planned 2026-10-30T18:27:52Z, `planned_by: human-fallback`) holds:

- Footprint and layers: L02, L03, L07.
- Story order: #990, #991, #992, #993, #994, #1028, #1106.
- Contracts (quoted):
  - "specs/&lt;f&gt;/baselines/&lt;part&gt;-v&lt;k&gt;.json and .draft.json (merged, #990-#994): unchanged"
  - "specs/&lt;story&gt;/plan.json gains a risk register (#1028): one entry per change a revert of the squash does
    not undo, each with its kind, what it changes, creates or destroys, its paths, and its rollback; aayu plan
    checks it like a Completion Goal"
  - "the risk: label of a story is set by aayu plan from its register: risk:high with an entry, else risk:low
    (Decision #1027)"
  - "v0.4.1: the failure-modes table of a state-changing high-risk story (#1106)"
- Rollback (quoted): "The register is a new optional field of plan.json; a revert of #1028 stops writing it, and a
  plan written with it still loads, since readers ignore an unknown field. No ledger record is rewritten."

## Layers touched

From `LAYERS.md`: L02 Interaction / API boundary (`internal/cli/**`, `internal/mcp/**`), L03 Application / workflow
(`internal/ready/**`, `internal/planning/**`, `internal/verbs/**`, `skills/**`), L07 Operational data / state
(`internal/config/**`, `internal/ledger/**`). The merged PRs also changed generated files from
`internal/verbs/registry.go` (`internal/mcp/schemas/ready.json`, `plan.json`, skill frontmatter) and from
`internal/config/defaults.toml` (`CONSTRAINTS.md`) (file lists of
[#1018](https://github.com/rahulnakmol/agentic-ayudha/issues/1018),
[#1021](https://github.com/rahulnakmol/agentic-ayudha/issues/1021),
[#1023](https://github.com/rahulnakmol/agentic-ayudha/issues/1023)).

## Delivered parts (code on `dev` at 884d81b)

| Concern | Code | Story |
|---|---|---|
| Baseline record and its check | `ready.Baseline`, `Baseline.Check`, `BaselinePath`, `Versions`, `BaselineSchema = "aayudha.feature.baseline/v1"` in `internal/ready/baseline.go` | #990 |
| Part owners | `partOwner` in `internal/ready/baseline.go`: intent and spec `product-strategist`, tech `code-architect`, ux `ux-researcher-evolutionist` | #990, #992 |
| Draft and approve | `runReadyPart`, `partRun.request`, `partRun.approve`, `partRun.holds`, `partRun.changed` in `internal/cli/ready.go` | #990 |
| Agent-lane refusal | `runReadyPart` returns `personOnly("ready --part")` when `AAYU_AGENT=1` or CI, before any client or ledger setup | #990 |
| Presence check | `presenceCheck(ctx, Pending{What: "ready --part", …})` in `partRun.approve` (`internal/cli/ready.go`) | #990 |
| Pins per part | `IntentPins`, `SpecPins`, `StoryPin`, `FilePin`, `ChartPin`, `EarlyBuildPin` in `internal/ready/pins.go`; `partRun.pins` in `internal/cli/ready.go` | #992 |
| Drift and reasons | `Drift`, `Diff`, `Change`, `Reasons`, `DraftHash`, `PartState`, `VersionWarning`, `Comment` in `internal/ready/drift.go`; `partRun.changes`, `baselineStates` in `internal/cli/ready.go` | #993 |
| Fixes of findings | `Baseline.Fixes`; `ready.Finding`, `ReviewPath`, `Review.Has` in `internal/ready/drift.go`; `partRun.fixes` in `internal/cli/ready.go` | #993 |
| Feature technical design | `DesignDraft`, `Design`, `DesignLedgerPath`, `CheckDesign`, `CheckInDesign` in `internal/planning/design.go`; `planFeature` and `featureDesign` in `internal/cli/plan.go` | #991 |
| Early build | `EarlyBuild`, `EarlyBuildPath`, `EarlyBuildBudget`, `EarlyBuildEffect`, `EarlyBuildHold` in `internal/ready/early.go`; `runReadyEarly`, `earlyStory` in `internal/cli/ready_early.go` | #994 |
| Early-build merge block | `ready.EarlyBuildLabel` checks in `internal/cli/inspect.go` and `internal/cli/inspect_app.go`; `EarlyBuildHold` refusal in `internal/cli/ship.go` | #994 |
| Config | `Baselines` struct (`why_min_words`, `version_warn`, `early_build_budget`) in `internal/config/config.go`; `[baselines]` in `internal/config/defaults.toml` | #993, #994 |

## Ledger records

| Path | Writer | Schema or shape |
|---|---|---|
| `specs/<f>/baselines/<part>-v<k>.draft.json` | the bot (agent), `ready --part --request` | `aayudha.feature.baseline/v1`, no `approved_by` |
| `specs/<f>/baselines/<part>-v<k>.json` | the person, `ready --part` | same schema, with `approved_by`, `approved_at`, `presence` |
| `specs/<f>/design.json` | the bot, `plan <feature> --draft <file> --create` | `planning.Design` |
| `specs/<f>/approved-early-<story>.json` | the person, `ready --early-build` | `aayudha.feature.early-build/v1` |
| `specs/<f>/reviews/<part>-v<k>/<seat>.json` | read only here (by `--fixes`); written by #875, not built | `ready.Review` |

## Config keys

| Key | Default | Source |
|---|---|---|
| `[baselines] why_min_words` | 5 | `internal/config/defaults.toml` |
| `[baselines] version_warn` | 4 | `internal/config/defaults.toml` |
| `[baselines] early_build_budget` | 1 | `internal/config/defaults.toml` |
| `[ready] dor_from` | planned `v0.4.1` | [#1106](https://github.com/rahulnakmol/agentic-ayudha/issues/1106) CG 8; planned, not built |

## Pending parts (planned, not built)

- **Risk register ([#1028](https://github.com/rahulnakmol/agentic-ayudha/issues/1028)).** The check goes in
  `planning.CheckPlan` (`internal/planning/plan.go`), beside the checkpoint and criteria checks, and the register
  adds to `em_review`. `aayu plan --create` then sets the `risk:` label. `rg risk internal/planning/plan.go`
  finds no register on `dev`. The ten kinds and the rule "`risk:` means rollback impact" are Decision
  [#1027](https://github.com/rahulnakmol/agentic-ayudha/issues/1027).
- **Failure modes and open questions ([#1106](https://github.com/rahulnakmol/agentic-ayudha/issues/1106)).** It
  adds a `failure_modes` field to the plan record, and a version gate `[ready] dor_from` read from the Feature's
  milestone. It files each `open_questions` entry as an ask (Feature
  [#880](https://github.com/rahulnakmol/agentic-ayudha/issues/880)) with a due time from Feature
  [#883](https://github.com/rahulnakmol/agentic-ayudha/issues/883) (milestone v0.4.1). The rules run in
  `aayu plan` and `aayu ready <story>` only, never in the Ready guard (F4). `open_questions` is a draft field
  today (`planning.PlanDraft.OpenQuestions` in `internal/planning/plan.go`), but the plan record does not keep it
  (#1106 Notes).

## Key decisions and deviations

- Decision [#1027](https://github.com/rahulnakmol/agentic-ayudha/issues/1027) (closed, done): inside an approved
  mission, the approval replaces the per-PR person, and `risk:` means rollback impact. It drives #1028.
- Decision [#593](https://github.com/rahulnakmol/agentic-ayudha/issues/593): #990 to #993 merged on proof through
  `aayu ship` (Outcome sections of those issues).
- Deviation recorded in PR [#1017](https://github.com/rahulnakmol/agentic-ayudha/issues/1017): a story-plan check
  without `--create` that cannot read the ledger passes with a warning (the plan evals run with no ledger); with
  `--create`, plan refuses when it cannot read the design. The code is in `runPlan` in `internal/cli/plan.go`.
- [#994](https://github.com/rahulnakmol/agentic-ayudha/issues/994) merged under a recorded solo-mode bypass, not
  on proof alone (its Outcome section).
