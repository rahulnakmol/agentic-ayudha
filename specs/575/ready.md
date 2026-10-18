# rahulnakmol/agentic-ayudha#575: An intent pass: a Story approved at Ready ships its PRs without a per-PR pass, after 50 sampled merges under 5 % corrections

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `d961772b266794fd211cac3965d7471a027b812a55d0a9a07c7bab10cc3bd2ad` |
| Completion Goal hash | `63eecc44aefea59160b89a3f62b78a08c1e3a4baf049d79b233ac10e75eb7ed5` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/575/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a person runs `aayu ready <n> --approve` on a Ready Story whose body matches its Ready snapshot; expected: the approval (login, body hash, time) is written to `specs/<n>/intent.json` on the ledger, and an agent lane is refused
2. [api-trace] state: `aayu ship <pr>` on a Story PR with no approving review, rung B at the head, an intent approval, an approved plan whose footprint covers the PR's, and no protected path, while fewer than 50 samples are recorded; expected: exit 4 with "intent pass held; relief is off (0 of 50 sampled merges)", the Ready to ship comment, and nothing merged
3. [api-trace] state: the same PR with 50 recorded samples and 2 corrections among them; expected: ship merges it, and the ship record's human pass is `intent` with the Ready approval and the plan review it rests on
4. [api-trace] state: the same PR with 50 samples and 3 corrections, or a PR whose footprint leaves its plan's, or a PR touching a protected path, or rung C; expected: no intent pass, and ship says which condition failed
5. [api-trace] state: `[autonomy] relief_samples` and `relief_max_correction_rate` in `internal/config/defaults.toml`; expected: 50 and 0.05, rendered in CONSTRAINTS.md, so only a protected-path change moves them

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want to approve a Story's intent once, at Ready (and its never-layer plan once), so that its PRs need no per-PR pass from me once enough sampled merges have shown that intent approval is safe.

Decision #572 option 1, story 2, with the founder's answer "after 50": the per-PR relief switches on only after 50 sampled merges that the intent pass would have let through show corrections under 5 %. Until then, ship records that the intent pass held and still stops for a person's pass (Ready to ship, batched in the inbox).

### Completion Goal

1. [api-trace] state: a person runs `aayu ready <n> --approve` on a Ready Story whose body matches its Ready snapshot; expected: the approval (login, body hash, time) is written to `specs/<n>/intent.json` on the ledger, and an agent lane is refused
2. [api-trace] state: `aayu ship <pr>` on a Story PR with no approving review, rung B at the head, an intent approval, an approved plan whose footprint covers the PR's, and no protected path, while fewer than 50 samples are recorded; expected: exit 4 with "intent pass held; relief is off (0 of 50 sampled merges)", the Ready to ship comment, and nothing merged
3. [api-trace] state: the same PR with 50 recorded samples and 2 corrections among them; expected: ship merges it, and the ship record's human pass is `intent` with the Ready approval and the plan review it rests on
4. [api-trace] state: the same PR with 50 samples and 3 corrections, or a PR whose footprint leaves its plan's, or a PR touching a protected path, or rung C; expected: no intent pass, and ship says which condition failed
5. [api-trace] state: `[autonomy] relief_samples` and `relief_max_correction_rate` in `internal/config/defaults.toml`; expected: 50 and 0.05, rendered in CONSTRAINTS.md, so only a protected-path change moves them

### Measure

criteria observed

### Threshold

5 of 5

### Scenario

Run the cli tests with a fake GitHub and a local ledger: a Story made Ready, its plan, its intent approval, and a gate-green PR.

### Environment

go test ./internal/cli with internal/github/fake

### Checkpoints

1. `aayu ready <n> --approve` writes the intent approval and refuses an agent lane (criterion 1)
2. ship computes the intent pass and stops with "relief is off" below 50 samples (criteria 2, 4, 5)
3. with 50 samples under 5 % corrections, ship merges on the intent pass and records it (criterion 3)

### Assumptions

- the samples are read from the ledger file that story 3 writes (`autonomy/samples.jsonl`); until story 3 lands, none exist, so relief stays off
- a PR touching a protected path keeps its per-PR pass until story 4 defines the narrow set
- the Story's author may approve its intent: an intent approval judges the Story and its plan, not the person's own diff

### Provenance

Request: "Build stories 2 to 4 in order, with the 50-sample gate as a config value in protected configuration."
Term: "after 50" → the per-PR relief switches on only after 50 sampled merges show corrections under 5 %; "The per-PR relief of option 1 switches on only after 50 sampled autonomous merges show corrections under 5 %"
Vague: "protected" → criterion 5: the values live in `internal/config/defaults.toml`, a CODEOWNERS path
1. stated (Decision #572 option 1): "The person approves each Story once, at Ready."
2. stated (founder, 2026-10-18): "until then, a person still passes per PR, batched"
3. stated (founder, 2026-10-18): "The per-PR relief of option 1 switches on only after 50 sampled autonomous merges show corrections under 5 %"
4. stated (Decision #572 option 1): "the PR's footprint stays inside the approved plan"
5. stated (founder, 2026-10-18): "with the 50-sample gate as a config value in protected configuration"
````
