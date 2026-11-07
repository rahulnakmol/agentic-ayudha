# Specification: #897 metered, risk-routed merge

No spec baseline exists (no `specs/897/` on `origin/ledger`), and no Story is filed, so no Completion Goal criteria
exist. Requirements trace to [#897](https://github.com/rahulnakmol/agentic-ayudha/issues/897) Scope and Decisions
[#902](https://github.com/rahulnakmol/agentic-ayudha/issues/902) and
[#910](https://github.com/rahulnakmol/agentic-ayudha/issues/910), and to a proposed slice in
[05-implementation-plan.md](05-implementation-plan.md) (proposed, not filed).

## Functional requirements

| ID | Requirement | Source | Proposed slice |
|---|---|---|---|
| FR1 | The tier is computed from the diff, with no model, and shown with its reason. | #897; #902 | 897-A |
| FR2 | High: a path in `[autonomy] always_person` or `always_person_config`; a path that `docs/threat-model.md` names; a workflow that reads a secret or names an environment. | #897; #902 Option 1 | 897-A |
| FR3 | Medium: a path under `skills/`, `seats/`, or `evals/`; a file that holds a model prompt; or more than 600 changed lines, not counting tests and generated files. Low: everything else. | #897; #902 Option 1 | 897-A |
| FR4 | The threat-model paths and prompt files are config lists: `[inspect.risk] high_paths` and `prompt_paths`. | #902 Option 1 | 897-A |
| FR5 | A workflow on push to `dev` writes one JSON line per merged PR on the ledger: PR, story, host, model, effort, tier, reviewer, rounds, fix rounds, tokens or cost, merged by, and outcome. The PR cannot write it. | #897 | 897-B |
| FR6 | A nightly sweep back-fills records from GitHub. A missing field stays empty and counts as a gap; it is never guessed. | #897 | 897-C |
| FR7 | The factory meter on Initiative #870 shows, each with source, unit, and age: merged per 28 days, fix-round hours, a person's wait, rework and reverts in 7 days, human touches per PR, and cost or tokens per PR and per story. | #897 | 897-D |
| FR8 | One review from the configured reviewer (the CI review, CodeRabbit, or another) on every tier; at most 3 rounds; after that a P2 or P3 is filed as a Bug, and a P0 or P1 follows the proof rule. | #897 (low-risk path), changed by #910 rules 1, 4, 5, 6 | 897-E |
| FR9 | Rollout: silent for 2 weeks, then visible, then enforced, with a kill switch in config. | #897 | 897-F |

## Non-functional requirements

| ID | Kind | Requirement | Source |
|---|---|---|---|
| NFR1 | Quality | No floor or check is skipped on any tier. | #897 Out; `CONSTRAINTS.md` ("Every floor miss blocks") |
| NFR2 | Quality | No LLM classifier gates. The tier is a fact from the diff. | #897 Out; #902 ("no model can raise or lower it (ETHOS 2)") |
| NFR3 | Data | A missing field is a gap, never a guess. | #897 |
| NFR4 | Data | Each meter number carries its source, unit, and age. | #897 |
| NFR5 | Security | The record is written by a workflow, not by the PR. | #897 |
| NFR6 | Operability | A kill switch in config turns enforcement off. | #897 |
| NFR7 | Cost | inspect spend on one PR stays under `budget.per_pr_inspect_max` (5.0 USD). | `CONSTRAINTS.md` |

## Points the sources do not settle

- #902 recommends Option 1, which drops today's high triggers for a new dependency and a migration. Option 2 keeps
  them. #902 says Option 1 was chosen ("do it"). The code still has the old triggers (`internal/inspect/risk.go`).
- What the tier gates, after #910 (review on every tier) and #1255 (merge on proof at any risk), is not restated in
  any source. #902 names the cold challenge of the whiteboard defense (#890) and the Review guidance (#864) as users.
- The meter's home is "Initiative #870". Whether it is a comment, a file, or an inbox view is not stated.
