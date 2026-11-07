# Technical design: #899 a finding blocks a merge only with proof

No technical baseline exists (no `specs/899/` on `origin/ledger`). Part 1 is what `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b) has. Part 2 is the planned design from
[#899](https://github.com/rahulnakmol/agentic-ayudha/issues/899) and Decision
[#910](https://github.com/rahulnakmol/agentic-ayudha/issues/910). Part 2 is **planned, not built**.

## Layers touched

| Layer | Paths (Expected paths of #899) |
|---|---|
| L02 | `internal/cli/**` |
| L03 | `internal/inspect/**`, `internal/delegate/brief.go`, `skills/build/**` |
| L04 | `internal/verify/**` |
| L05 | `evals/**` |
| L07 | `internal/ledger/**`, `internal/config/defaults.toml` |
| L09 | `.github/workflows/aayudha-merge-gate.yml` |
| (no layer) | `AGENTS.md`, `docs/threat-model.md`, `docs/guide/how-merge-gate.md` |

## Part 1: what exists on dev

| Building block | Where | What it does |
|---|---|---|
| Finding | `internal/inspect/inspect.go:63-88` (`Finding`: `ID`, `Severity`, `Category`, `File`, `Line`, `Quote`, `Title`, `Detail`, `Fix`, `Confidence`, `Criterion`, `Sources`, `Disputed`, `Class`, `Instances`, `Repeat`) | `Fix` is advice text. There is no probe and no patch field. |
| Rounds | `internal/inspect/inspect.go:53` (`MaxRounds = 3`), `ReopenCommand = "/aayu review again"`, `State.Capped` (line 357) | Three judged rounds, then `step-back`, reopened only by a person. |
| Carried findings | `internal/inspect/inspect.go` (`carried`, `carryForward`, lines 748-768) | A finding on an unchanged patch stays open. |
| Disposition and floors | `internal/inspect/inspect.go` (`Run` line 455, `begin` 696, `finish` 777, `Enforce` 973) | `Enforce` applies the proof pack to the result. |
| Reviewer choice | `internal/inspect/apps.go:139` (`ChooseReviewer`), `[inspect] reviewer = "ci"` (`internal/config/defaults.toml`) | One configured reviewer, with a fallback order. |
| Confidence filter | `[inspect] min_confidence = 0.5` (`internal/config/defaults.toml`) | P2 and P3 findings below it are not posted. |
| Dismissals | `aayu inspect <pr> --dismiss <id> --reason …`; ledger `inspect/dismissed.jsonl` | A person dismisses a false finding. |
| Recent inspect fixes | [18845b72](https://github.com/rahulnakmol/agentic-ayudha/commit/18845b72) (#1135, CodeRabbit icon badges), [1fe6b53a](https://github.com/rahulnakmol/agentic-ayudha/commit/1fe6b53a) (#1156), [b7b36fda](https://github.com/rahulnakmol/agentic-ayudha/commit/b7b36fda) (#1180), [d40b7c9b](https://github.com/rahulnakmol/agentic-ayudha/commit/d40b7c9b) (#1145) | How findings are read and carried. |
| Merge gate | `.github/workflows/aayudha-merge-gate.yml` | Runs inspect on each head. In `[autonomy] always_person`. |
| Sandbox | — | The evidence does not show a sandbox that runs a reviewer's probe. `aayu install --env --verify` checks environments in "a clean container" (`aayu install --help`), which is a different purpose. |

## Part 2: planned design (planned, not built)

1. **Replay (FR1).** Replay PR #828's rounds under the proof rule. #910 reports a replay result already (6 of 8
   findings had a probe, an upper bound). #899 asks for the measure as part of the Feature. Inferred: the replay
   becomes a repeatable eval case under `evals/cases/inspect/`.
2. **Probe or patch on a finding (FR2, FR3).** Add a probe (a test that should fail on the head) and a candidate patch
   to `Finding`, and a prover pass that writes a probe for each P0 or P1 (#910 rule 6).
3. **Sandbox run (FR4).** Run the probe on the PR head, and the patch with the tests, in an isolated job. Where it runs
   is not stated. Inferred: a job of the merge gate, because the gate already holds the head.
4. **Block rule (FR5 to FR7).** In `finish` / `Enforce`: a P0 or P1 blocks only while its probe fails on the head; a
   P0 on a CODEOWNERS path always blocks; otherwise the finding becomes its own Bug and the disposition can be
   `merge` (#910 rule 6).
5. **Follow-ups (FR8).** After `merge`, a new P2 or P3 or out-of-story finding is listed under "Follow-up" and filed as
   one Bug (#910 rule 2).
6. **Precision (FR9).** Each validated or unvalidated finding is counted into #897's merge record.
7. **RAID rule (FR10).** Update `AGENTS.md`, `skills/build/**`, and `docs/guide/how-merge-gate.md` so a declined
   finding needs no RAID row. `aayu inspect` checks the PR body (`AGENTS.md`), so its check may change too. Inferred.

## Decisions

- [#910](https://github.com/rahulnakmol/agentic-ayudha/issues/910) (done): the seven review rules, rule 6 is this
  Feature's core. It supersedes #905 (#910 Links; #905 is `status:cancelled`). #902 says the review of every tier
  "replaces #894", but Decision #894 is still open with `status:planned` (GitHub, 2026-11-07).
- [#902](https://github.com/rahulnakmol/agentic-ayudha/issues/902) (done): risk tiers, used by #897.
- D9 (issue text). Not in this repository. Unverified.
