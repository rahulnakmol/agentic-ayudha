# Spec: Feature #874

## Spec baseline

`specs/874/baselines/spec-v1.json` on `ledger`: owner `product-strategist`, approved by `rahulnakmol` at
2026-10-30T19:10:05Z through `device-flow`, hash `387053ead9e2…`. It pins seven stories by their title, Completion
Goal hash, effort, layers, and depends-on numbers, plus one pin `#874 stories` for the set of story numbers:

| Story | Effort | Layers (pinned) | Depends on (pinned) |
|---|---|---|---|
| [#990](https://github.com/rahulnakmol/agentic-ayudha/issues/990) | L | L02, L03 | none |
| [#991](https://github.com/rahulnakmol/agentic-ayudha/issues/991) | M | L02, L03 | none |
| [#992](https://github.com/rahulnakmol/agentic-ayudha/issues/992) | M | L02 | #990, #991 |
| [#993](https://github.com/rahulnakmol/agentic-ayudha/issues/993) | M | L02, L03, L07 | #992 |
| [#994](https://github.com/rahulnakmol/agentic-ayudha/issues/994) | S | L02, L03, L07 | #990 |
| [#1028](https://github.com/rahulnakmol/agentic-ayudha/issues/1028) | M | none | none |
| [#1106](https://github.com/rahulnakmol/agentic-ayudha/issues/1106) | M | none | #883, #1028 |

## Functional requirements

Each requirement names its Story and the Completion Goal criteria (CG) it comes from.

| # | Requirement | Story, CG | State |
|---|---|---|---|
| FR1 | `aayu ready <f> --part <p> --request` writes `specs/<f>/baselines/<p>-v<k>.draft.json` (part, version, owner seat, pins, hash, summary, source session) and one marker comment `<!-- aayudha:baseline part=<p> -->` that says the draft waits for a person. | #990 CG 1, 2 | delivered |
| FR2 | `aayu ready <f> --part <p>` without `--request` is refused for an agent lane (`AAYU_AGENT=1`) with exit 4, before any ledger read or write. | #990 CG 3 | delivered |
| FR3 | A person approves at a terminal through GitHub's device flow. The approval writes `<p>-v<k>.json` with the draft's hash, `approved_by`, `approved_at`, and `presence: device-flow`. | #990 CG 4 | delivered |
| FR4 | The approval reads the pins again after the device code. When they moved, nothing is baselined. When no draft waits, exit 4 names the `--request` command. | #990 CG 5, 6 | delivered |
| FR5 | `aayu plan <feature>` reports the footprint and the stories and writes nothing. `--draft` checks layers against the footprint, the story order against the child stories, and a rollback when L07 is touched. `--create` writes `specs/<f>/design.json`. | #991 CG 1-5 | delivered |
| FR6 | A story plan whose checkpoint layer is outside its Feature's `design.json` fails. A story whose Feature has no design passes as before. | #991 CG 6, 7 | delivered |
| FR7 | The spec part pins each child story by title, Completion Goal hash, effort, layers, and depends-on, and pins the set of story numbers. An edit to other text of a story does not change a pin. | #992 CG 1, 2 | delivered |
| FR8 | The tech part pins `design.json` and each chart comment by SHA-256, and is refused when no `design.json` exists. | #992 CG 3, 4 | delivered |
| FR9 | The UX part is refused when no Expected path is in L01. Otherwise it pins each stage manifest and the head SHA of each open early-build PR. | #992 CG 5, 6 | delivered |
| FR10 | `aayu ready <f> --check` on a Feature shows a Baselines block with each part's drift, computed from the pins at read time; nothing writes a drift record. | #993 CG 1 | delivered |
| FR11 | A version after v1 needs `--changes` with what, why, and need for each drift, each of at least `[baselines] why_min_words` words; the draft hash covers the changes. A no-op request writes nothing. | #993 CG 2-6 | delivered |
| FR12 | A draft at `[baselines] version_warn` or later warns. | #993 CG 7 | delivered |
| FR13 | `--fixes <ids>` names findings of the same part that exist on the ledger, inside the hash; another part's finding or an unknown ID exits 2. | #993 CG 8 | delivered |
| FR14 | `[baselines]` keys default to `why_min_words = 5` and `version_warn = 4`, and `baselines` is in `[autonomy] always_person_sections`. | #993 CG 9 | delivered |
| FR15 | `aayu ready <f> --early-build <story>` approves one child story labelled `early-build`, shows the budget (`[baselines] early_build_budget` × `[autonomy.budget] cost_usd`) before the device code, and writes `specs/<f>/approved-early-<story>.json`. | #994 CG 1, 3 | delivered |
| FR16 | An early build is refused before the device code when the story has no label or is not a child; refused for an agent lane. | #994 CG 2, 5 | delivered |
| FR17 | `aayu inspect` holds, and `aayu ship` refuses, a PR that refs an early-build story: "its PR never merges into dev (E1)". | #994 CG 4 | delivered |
| FR18 | `aayu plan <story> --draft` refuses a plan with no `risks` key, an unknown kind (ten kinds from Decision #1027), an entry with no changes, paths, affects, or rollback, and a rollback that is only "revert the PR". | #1028 CG 1-3, 8 | not built |
| FR19 | `--create` writes `risks` and `risks_sha256` into `specs/<story>/plan.json`, sets `risk:high` with an entry and `risk:low` with an empty register, and adds the entries to `em_review`. | #1028 CG 4, 5 | not built |
| FR20 | A plan with a register can be created before the Feature's Approve to build. While a mission lock holds, the plan is written but the label is not. | #1028 CG 6, 7 | not built |
| FR21 | A plan whose register has a contract, migration, or external-write entry needs a `failure_modes` row per state-changing operation (idempotency key, dedup window, retry, duplicate, out-of-order, outbox). | #1106 CG 1-3 | not built |
| FR22 | The rule applies only from `[ready] dor_from` (default `v0.4.1`), read from the Feature's milestone; v0.4.0 work keeps today's rules, and the Ready guard never applies it. | #1106 CG 4, 5, 8 | not built |
| FR23 | `aayu ready <story> --check` prints one Definition of Ready line. Each `open_questions` entry is filed as an ask with an SLA due time, and Ready is refused while one is open. | #1106 CG 6, 7 | not built |

## Non-functional requirements

| # | Requirement | Source |
|---|---|---|
| NFR1 | Security: only a person approves a baseline or an early build, through GitHub's device flow, every time. An agent lane is refused before any write. | #990 Behaviours; #994 CG 5; threat T19 in `docs/threat-model.md` |
| NFR2 | Integrity: a person approves only the hash they saw. The pins are read again after the device code. | #990 Review guidance (check 5); `partRun.approve` in `internal/cli/ready.go` |
| NFR3 | Security: agent text in what, why, and need is untrusted (threat T6). The marker comment escapes it. | #993 Assumptions; `ready.Comment` and `cell` in `internal/ready/drift.go` |
| NFR4 | Operability: drift is computed when read. Nothing polls and nothing writes a drift record. | #993 Behaviours 2 |
| NFR5 | Configurability: every limit is a config key with a default (`why_min_words`, `version_warn`, `early_build_budget`), and a change to `[baselines]` needs a person. | #993 Behaviours 3; `internal/config/defaults.toml` `[baselines]` and `[autonomy] always_person_sections` (lists `baselines`); `.aayudha/config.toml` `always_person_config` |
| NFR6 | Cost: an early build covers one story and shows its budget before approval. | #994 Behaviours 1 |
| NFR7 | Test environment: every criterion runs on the fake GitHub (`internal/github/fake`) and a local ledger, never production. | Environment sections of #990-#994, #1028, #1106 |
| NFR8 | Reliability: a reverted #1028 leaves plans readable, because readers ignore an unknown field. | `specs/874/design.json` rollback |
