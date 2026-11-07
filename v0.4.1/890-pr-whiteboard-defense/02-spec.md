# #890 Spec

No spec baseline exists (`specs/890` is absent from the ledger). FR1 to FR10 trace to Stories #1107 and #1108 and
their Completion Goal criteria. FR11 to FR17 trace to #890 and Decision #904 and have no Story yet; the "Slice"
column names a proposed slice from [05-implementation-plan.md](05-implementation-plan.md) (proposed, not filed).

## Functional requirements

| # | Requirement | Source | Story or slice |
|---|---|---|---|
| FR1 | The `contracts` floor fails an undeclared breaking change to a declared contract path, with the path and the removed field. | [#1107](https://github.com/rahulnakmol/agentic-ayudha/issues/1107) criterion 1 | #1107 |
| FR2 | A breaking change passes when the story's risk register has a contract entry whose paths name the changed file; the pack records "breaking change declared as risk 1". | #1107 criterion 2 | #1107 |
| FR3 | An optional field passes; a PR with no contract path says "no contract changed". | #1107 criterion 3 | #1107 |
| FR4 | The `contract-docs` floor runs `[verify.floors.contracts] docs_command` in the verify sandbox and fails with the files that differ. | #1107 criterion 4 | #1107 |
| FR5 | Defaults are `paths = []` and `docs_command = []`, reported "skipped: no contracts declared". A miss is advisory before `[verify] dod_floors_blocking_from`, then fails the gate. | #1107 criterion 5 | #1107 |
| FR6 | The `boundaries` floor fails an import that `[verify.floors.boundaries] inward_only` forbids, naming both layers. | [#1108](https://github.com/rahulnakmol/agentic-ayudha/issues/1108) criterion 1 | #1108 |
| FR7 | A repository that is not a Go module gets "skipped: boundaries are read from Go imports in v0.4.1". | #1108 criterion 2 | #1108 |
| FR8 | The `fixtures` floor fails one finding per email, Luhn-valid card, or secret in a fixture, with the value redacted. | #1108 criterion 3 | #1108 |
| FR9 | Safe test values and `[verify.floors.fixtures] allow` entries pass; the floor scans every file under its paths at the head, not only the diff. | #1108 criteria 4, 5 | #1108 |
| FR10 | Fixture defaults: `paths = ["**/testdata/**", "**/fixtures/**", "**/*_test.go"]`, `kinds = ["email", "card", "phone", "secret"]`, `allow = []`; boundary default `inward_only = []`. | #1108 criterion 6 | #1108 |
| FR11 | ETHOS.md gets principle 11, whiteboard defense. | #890 Scope; #904 option 1 | P1 |
| FR12 | The PR template gets a Defense section with five parts: how it works, why X and not Y, a malicious actor (citing threat-model rows by ID), data structures, where it fails. It also gets the DoD checklist naming the floors of #1107 and #1108. | #890 Scope and DoD note; #904 option 1 | P1 |
| FR13 | A missing or uncited Defense blocks through inspect's deterministic PR-body check, for a person's PR and an agent's. A docs-only or generated-only PR is exempt; a bot-authored PR only warns. | #904 option 1 | P2 |
| FR14 | The cold challenge blocks a PR whose risk tier is at or above `[inspect] challenge_tier` (default `"high"`). A fresh session reads only the diff and the Defense, asks four questions with one follow-up each, inside `[budget] per_pr_inspect_max`. | #904 option 1 | P3 |
| FR15 | Every other merged PR has a 1-in-10 cold challenge after merge; a failure opens a Bug. | #904 option 1 | P4 |
| FR16 | The Defense is copied to the ledger. | #890 Scope ("the ledger copy") | P4 |
| FR17 | `aayu explain <pr\|path> "<question>"` asks the seat that owns the code; "I do not know" opens a Bug on the defense. | #904 option 1 | P5 |

## Non-functional requirements

| Kind | Requirement | Source |
|---|---|---|
| Security | The fixtures report never holds a found value, only its kind and place. | #1108 Review guidance |
| Security | The docs command runs in the verify sandbox, never on the host. | #1107 Review guidance |
| Reliability | A new floor is advisory until a person sets its blocking date. | #1107 Behaviour 3; pattern of `newFloorAdvisory` (`internal/verify/dast.go:331-337`) |
| Reliability | A floor must not fail every PR of a repository with no contracts. | #1107 Review guidance, Risk line |
| Cost | The cold challenge runs inside `[budget] per_pr_inspect_max`; the default tier `high` limits it to high-risk PRs. | #904 Evaluation |
| Operability | The layer of a file comes from LAYERS.md's globs, the same as slice. | #1108 Review guidance |
| Determinism | A model judgement may block, but never override a floor. | #865 Evaluation, citing ETHOS principle 2 |

## Open questions

- Decision #902's risk tiers decide the challenge. The evidence does not show them built: `inspect.RiskOf`
  (`internal/inspect/risk.go:45`) still uses the P6 thresholds (3 layers, 400 lines, 20 files) and names no
  threat-model path. See [03-technical-design.md](03-technical-design.md).
- #1107 needs a "contract entry" in a story's risk register, which Story #1028 builds. #1028 is `status:planned`.
