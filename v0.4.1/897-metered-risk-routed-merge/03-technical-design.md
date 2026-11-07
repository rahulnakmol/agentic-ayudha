# Technical design: #897 metered, risk-routed merge

No technical baseline exists (no `specs/897/` on `origin/ledger`). Part 1 is what `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b) has. Part 2 is the planned design from
[#897](https://github.com/rahulnakmol/agentic-ayudha/issues/897) and Decisions
[#902](https://github.com/rahulnakmol/agentic-ayudha/issues/902) and
[#910](https://github.com/rahulnakmol/agentic-ayudha/issues/910). Part 2 is **planned, not built**.

## Layers touched

| Layer | Paths (Expected paths of #897) |
|---|---|
| L02 | `internal/cli/ship.go` |
| L03 | `internal/inspect/risk.go`, `internal/inspect/`, `internal/measure/` |
| L07 | `internal/ledger/`, `internal/config/defaults.toml` |
| L09 | `.github/workflows/` |

## Part 1: what exists on dev

| Building block | Where | What it does |
|---|---|---|
| Risk tier | `internal/inspect/risk.go:45` (`RiskOf`), constants `riskLayers = 3`, `riskLines = 400`, `riskFiles = 20` | High: a never-layer footprint, a CODEOWNERS-protected path, a changed dependency manifest, or a migration. Medium: 3 or more layers, more than 400 lines, or more than 20 files. Added in [469adabb](https://github.com/rahulnakmol/agentic-ayudha/commit/469adabb) (#228). "It approves nothing and changes no gate" (`Risk` comment). |
| Tier caller | `internal/cli/proof.go:728` | The proof comment's risk line. |
| Ship record | `internal/cli/ship.go:66` (`ShipRecord`: repo, story, PR, head, merge commit, pack, gate run, human pass, proof, review rung, footprint, streak) at `proof/<story>/<head>/authoritative/ship.json` | Written by `aayu ship` for a Story PR. Not one record per merged PR, and no host, model, rounds, or cost fields. |
| Audit entry | `internal/cli/inboxaudit.go` (`AuditLedgerPath` = `autonomy/audit/<pr>.json`, `AuditEntry`, `SamplesLedgerPath` = `autonomy/samples.jsonl`) | A PR merged with no person's pass waits for a person's look. |
| Founder minutes | `internal/measure/founder.go` (`Founder`, `FounderMetric = "founder_minutes_per_pr"`), [2e62c537](https://github.com/rahulnakmol/agentic-ayudha/commit/2e62c537) (#1016) | A person's minutes per merged PR, from the actions log. A meter input ("a person's wait"). |
| Streak | `aayu status --streak` ("Compute the autonomy streak of every layer from merged-PR history", `aayu status --help`) | A meter input ("human touches"). Inferred. |
| Rounds | `internal/inspect/inspect.go:53` (`MaxRounds = 3`), `State.Capped` | Three judged rounds, then `step-back`. #910 changes this to "only a fix round counts" and "then Bugs". |
| Reviewer choice | `internal/inspect/apps.go:139` (`ChooseReviewer`); `[inspect] reviewer = "ci"` | One configured reviewer with a fallback order. |
| Agent PR trailers | `Aayudha-Host`, `Aayudha-Model`, `Aayudha-Effort` (`AGENTS.md`, Branches, PRs, commits) | The source of host, model, and effort fields. |
| Narrow set | `.aayudha/config.toml:75-102` (`always_person`, `always_person_config`) | High-tier input in #902. |

## Part 2: planned design (planned, not built)

1. **Tier (FR1 to FR4).** Rewrite `RiskOf` to #902 Option 1: high from `always_person` and `always_person_config`,
   `[inspect.risk] high_paths` (default: paths that `docs/threat-model.md` names), and credential workflows; medium
   from `skills/`, `seats/`, `evals/`, `[inspect.risk] prompt_paths`, and more than 600 changed lines excluding tests
   and generated files; low otherwise. Each matched rule is a reason (#902).
2. **Merge record (FR5).** A workflow on `push` to `dev` appends one JSON line per merged PR to a ledger file. The path
   is not stated. The fields are #897's list. The workflow writes as the bot (the ledger's writers are "Only CI and
   the bot identity", `origin/ledger:README.md`).
3. **Back-fill (FR6).** A nightly job reads GitHub for each merged PR with no record, and writes empty fields as gaps.
4. **Meter (FR7).** `internal/measure/` computes the meter numbers from the records, with source, unit, and age, and
   posts them on Initiative #870. Inferred: in the pattern of `aayu status --apply` "replace-in-place" comments.
5. **Review path (FR8).** One configured reviewer, the 3-round rule, and Bugs after round 3, on every tier (#910).
6. **Rollout (FR9).** A config switch with three states (silent, visible, enforced) and a kill switch. The key name is
   not stated.

## Config keys (planned)

- `[inspect.risk] high_paths`, `[inspect.risk] prompt_paths` (#902).
- A rollout state and kill switch (#897). Name not stated.

## Decisions

- [#902](https://github.com/rahulnakmol/agentic-ayudha/issues/902) (done): tier rules.
- [#910](https://github.com/rahulnakmol/agentic-ayudha/issues/910) (done): review on every tier, 3 rounds then Bugs.
- [#1255](https://github.com/rahulnakmol/agentic-ayudha/issues/1255) (done): merge on proof at any risk; draft PR
  [#1282](https://github.com/rahulnakmol/agentic-ayudha/issues/1282) built it, but it closed unmerged on 2026-11-07 when the mission ended; branch `feat/1281-auto-merge-on-proof` at `6f2aae7` kept (`_data/closed-prs.tsv`). Nothing of
  #1255 is on `dev`.
- D2 and D3 (issue text). Not in this repository. Unverified.
