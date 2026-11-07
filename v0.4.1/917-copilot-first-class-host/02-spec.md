# Specification: #917 GitHub Copilot as a first-class host

No spec baseline exists (no `specs/917/` on `origin/ledger`, 2026-11-07), and the Feature has no Stories, so no
Completion Goal criteria exist yet. Each requirement below traces to a Scope item of
[#917](https://github.com/rahulnakmol/agentic-ayudha/issues/917) and to a proposed Story slice in
[05-implementation-plan.md](05-implementation-plan.md) (marked "proposed, not filed").

## Functional requirements

| ID | Requirement | Source | Proposed slice |
|---|---|---|---|
| FR1 | The Copilot build lane defaults to Auto Balance. | #917 C1 | 917-A |
| FR2 | The Copilot design, review, and UI review lanes default to Auto Intelligence (G3). | #917 C1 | 917-A |
| FR3 | Copilot loops run on Auto Efficiency. | #917 C1 | 917-A |
| FR4 | A person can still name a model per lane with `[models.<lane>.copilot] model`. | #917 C1 | 917-A |
| FR5 | `aayu models show --host copilot` shows the tier per lane. The guide's "Lanes and models" section says why, with the cost note. | #917 C1 | 917-A |
| FR6 | aayu records the model that served each Copilot review. | #917 C2 | 917-B |
| FR7 | A Copilot Auto review counts for merge on proof with no family check (G2). Every other host keeps the cross-family rule. | #917 C2 | 917-B |
| FR8 | `aayu build --delegate copilot --local` runs `copilot -p` with the brief in the story worktree, at the lane's tier. | #917 C3 | 917-D |
| FR9 | The local receiver's environment has the Copilot token only, never aayu's GitHub token. | #917 C3, Problem | 917-C |
| FR10 | A local follow-up uses `--resume`. Trailers come from the session log. | #917 C3 | 917-D |
| FR11 | `aayu build --delegate copilot` starts a cloud task on `dev` through the REST tasks API. A follow-up is a new task on the PR's head branch. | #917 C4 | 917-F |
| FR12 | The cloud hand-off sends `model: "auto"` when a live probe shows the API takes it, else no model. It records the task's model and usage in the trailers and the spend record. | #917 C4 | 917-F |
| FR13 | `aayu install --host copilot` writes `copilot-setup-steps.yml` and names the secret to set. | #917 C4 | 917-E |
| FR14 | On a plan other than Business or Enterprise, `aayu doctor` and `aayu build` refuse with a clear message (G5). | #917 C4 | 917-E |

## Non-functional requirements

| ID | Kind | Requirement | Source |
|---|---|---|---|
| NFR1 | Security | aayu's GitHub token never reaches a Copilot receiver. | #917 Problem and C3 |
| NFR2 | Security | The Copilot coding agent bot is never assigned a story issue, so it never reads an untrusted issue body as its brief. | #917 Out |
| NFR3 | Security | The receiver never gets merge rights. aayu opens the PR, inspect reviews it, and ship merges it. | `internal/delegate/state.go` package comment (applies to every receiver) |
| NFR4 | Reliability | Each hand-off and follow-up to a receiver ends after two minutes. | [7239cb04](https://github.com/rahulnakmol/agentic-ayudha/commit/7239cb04) (#1136), applies to every receiver |
| NFR5 | Cost | The guide states a cost note for the Auto tiers. The registry records that an Auto tier once picked a model at twice the per-token price (`internal/verbs/registry.go:453-455`). | #917 C1; code comment |
| NFR6 | Cost | Model spend stays under `budget.per_story_usd` (60.0) and `autonomy.budget.cost_usd` (60.0) where aayu's calls are pay-as-you-go. | `CONSTRAINTS.md` |
| NFR7 | Operability | A plan that cannot run cloud tasks gets a refusal, not a silent failure. | #917 C4 (G5) |
| NFR8 | Quality | The changed-line coverage floor is 90.0 percent. | `CONSTRAINTS.md`, `verify.floors.coverage.min_diff_percent` |

## Open points the sources do not settle

- Whether the REST tasks API accepts `model: "auto"`. The issue makes it conditional on a live probe (C4). Unverified.
- How aayu detects the person's Copilot plan for FR14. The issue does not say. Unverified.
- C2 is an exception to the cross-family rule that `AGENTS.md` states ("a review by a model of another family"). The
  issue says the founder made the exception (G2). The decision record is not in this repository. Unverified.
