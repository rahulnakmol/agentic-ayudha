# #890 Technical design

No tech baseline exists (`specs/890`, `specs/1107`, `specs/1108` are absent from the ledger). Everything under
"Planned" is planned, not built. It comes from #890, Decision #904, and the Notes of #1107 and #1108.

## What exists on dev that this builds on

All paths are at `dev` [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b).

| Part | Where | What it does now |
|---|---|---|
| PR template | `.github/pull_request_template.md`; embedded copy `internal/prbody/pull_request_template.md` (`//go:embed` at `internal/prbody/check.go:17`) | Six sections; no Defense. |
| PR-body check | `prbody.Check` (`internal/prbody/check.go:199`) | Blocks a missing, empty, or placeholder section and a sentence over 20 words. |
| Merge-gate floor | `Env.prBodyFloor` (`internal/cli/inspect_app.go:578-`) | Turns `merge` into `fix` on an agent PR (label `agent-authored` or `human-fallback`, or an `Aayudha-Host` line). A bot author only warns. A person's PR is not blocked. |
| Risk tier | `inspect.RiskOf` (`internal/inspect/risk.go:45`), thresholds `riskLayers = 3`, `riskLines = 400`, `riskFiles = 20` | The proof comment's risk line (P6), from PR #228, commit [469adab](https://github.com/rahulnakmol/agentic-ayudha/commit/469adabb). The file has no later commit, so the evidence does not show Decision #902's tiers built. |
| Verify floors | `verify.Floor` interface (`internal/verify/verify.go:83-90`); `[verify.floors]` (`config.Verify.Floors`, `internal/config/config.go:408`) | Each floor has `Name`, `Applies`, `Run`. |
| Advisory window | `newFloorAdvisory` (`internal/verify/dast.go:331-337`); `[verify] new_floors_blocking_from = "2026-11-14"` (`internal/config/defaults.toml:20`) | A new floor's miss is advisory before its date. #1107 copies this for `dod_floors_blocking_from`. |
| Secrets floor | `Secrets.Run` (`internal/verify/scanners.go:273-328`) | Scans the PR's commits only; #1108 adds files at the head. |
| Layer globs | `LAYERS.md`, generated from `internal/verbs/registry.go`; footprint in `internal/verify/footprint.go` | Maps paths to L01 to L09. LAYERS.md states no import rule, so #1108 makes the rule config. |
| Threat model | `docs/threat-model.md` (rows T1 and on, for example T7 at line 62) | The rows the Defense's "malicious actor" part cites. |
| Narrow set | `.aayudha/config.toml` `[autonomy] always_person` | Paths that always need a person per PR. |

Not on `dev`: an `explain` verb (no `Name: "explain"` in `internal/verbs/registry.go`; `aayu explain --help` prints
the root help), `internal/cli/explain.go`, `.github/workflows/aayudha-defense.yml`, a `[inspect] challenge_tier` key,
a `[verify] dod_floors_blocking_from` key, and the `contracts`, `contract-docs`, `boundaries`, and `fixtures` floors.

## Layers touched

From `LAYERS.md`: L02 (`internal/cli/**`), L03 (`internal/prbody/**`, `internal/inspect/**`, `internal/delegate/**`,
`internal/initrepo/**`, `internal/verbs/**`), L04 (`internal/verify/**`, `CONSTRAINTS.md`), L07
(`internal/config/**`), L09 (`.github/**`). `ETHOS.md` and `docs/threat-model.md` are in no layer.

## Planned, not built

1. **DoD floors (#1107, #1108).** New `verify.Floor` values `contracts`, `contract-docs`, `boundaries`, `fixtures`
   with keys under `[verify.floors.contracts]`, `[verify.floors.boundaries]`, `[verify.floors.fixtures]`, and a
   `[verify] dod_floors_blocking_from` date. Compatibility rules cover JSON Schema and OpenAPI in v0.4.1; other
   formats report "skipped: no compatibility rules for `<format>`" (#1107 Notes). #1107 Notes ask that #890's Expected
   paths gain `internal/verify/**`, `internal/config/**`, and `CONSTRAINTS.md`; the current body lists them.
2. **Defense text (P1).** ETHOS principle 11 and the five-part Defense section in both template copies, plus the DoD
   checklist (#890 DoD note).
3. **Defense check (P2).** Extend `prbody.Check` and `prBodyFloor` so a missing or uncited Defense blocks every PR into
   `dev`, a person's included. "Cites code" means each part cites code that exists at the PR head (#904).
4. **Cold challenge (P3).** In the merge gate, for a tier at or above `[inspect] challenge_tier`. Sanjay (the Mentalist
   seat's twin) reads only the diff and the Defense in a fresh session. An unsupported answer is a finding that goes
   through the fix loop (#904). Files from #890: `internal/inspect/**`, `internal/cli/inspect.go`,
   `internal/cli/inspect_app.go`, `internal/config/config.go`, `internal/config/defaults.toml`.
5. **Spot check and ledger copy (P4).** A 1-in-10 cold challenge after merge, run from
   `.github/workflows/aayudha-defense.yml`; a failure opens a Bug. The ledger path for the copy is not named in any
   source.
6. **`aayu explain` (P5).** A new verb in `internal/verbs/registry.go` and `internal/cli/explain.go`.

## Key decisions

| Decision | State | Effect |
|---|---|---|
| [#865](https://github.com/rahulnakmol/agentic-ayudha/issues/865) | cancelled, superseded by #904 | The first form: challenge at risk medium or high. |
| [#904](https://github.com/rahulnakmol/agentic-ayudha/issues/904) | accepted 2026-10-23 | The defense of #865 with `[inspect] challenge_tier`, default `"high"`. |
| [#902](https://github.com/rahulnakmol/agentic-ayudha/issues/902) | closed `status:done` | Deterministic risk tiers from the diff; the challenge uses them. Not visible in `internal/inspect/risk.go`. |
| [#1027](https://github.com/rahulnakmol/agentic-ayudha/issues/1027) | closed `status:done` | "the register is the declaration" (#1107 Review guidance: settled). |

## Dependencies

Issue #890 links `depends-on` [#887](https://github.com/rahulnakmol/agentic-ayudha/issues/887) (Review guidance; Stories #1221 to #1223 planned), [#897](https://github.com/rahulnakmol/agentic-ayudha/issues/897) (v0.4.1, not sliced),
[#880](https://github.com/rahulnakmol/agentic-ayudha/issues/880) (not sliced), and
[#871](https://github.com/rahulnakmol/agentic-ayudha/issues/871) (not sliced). #1107 depends on
[#1028](https://github.com/rahulnakmol/agentic-ayudha/issues/1028) (risk register, `status:planned`). #1108 depends
on #1107.
