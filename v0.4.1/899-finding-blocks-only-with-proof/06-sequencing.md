# Sequencing: #899 a finding blocks a merge only with proof

The slices are the proposed ones in [05-implementation-plan.md](05-implementation-plan.md) (proposed, not filed).

## Before any build

1. File the Stories (`aayu slice 899`). A person approves the baselines and gives Approve to build
   ([af81cb48](https://github.com/rahulnakmol/agentic-ayudha/commit/af81cb48), #1018;
   [45a18e74](https://github.com/rahulnakmol/agentic-ayudha/commit/45a18e74), #1024).
2. A person chooses the sandbox (899-C).
3. #897's merge record must exist before 899-F.

## Waves

| Wave | Lane | Slice | Files or packages it edits |
|---|---|---|---|
| 1 | 1 | 899-A replay of #828 | `evals/cases/inspect/` (new case), `internal/evals/review.go` |
| 1 | 2 | 899-B probe or patch on a finding | `internal/inspect/inspect.go` (`Finding`), reviewer prompt files |
| 2 | 1 | 899-C sandbox run | `internal/verify/**` (a new file), `.github/workflows/aayudha-merge-gate.yml` |
| 2 | 2 | 899-E follow-ups | `internal/inspect/inspect.go` (`finish`), `internal/cli/inspect.go` |
| 3 | 1 | 899-D block rule | `internal/inspect/inspect.go` (`finish`, `Enforce`), `internal/cli/inspect.go` |
| 3 | 2 | 899-H threat model | `docs/threat-model.md` |
| 4 | 1 | 899-F precision in the merge record (after #897) | `internal/ledger/**`, #897's record writer |
| 4 | 2 | 899-G RAID rule | `AGENTS.md`, `skills/build/**`, `internal/delegate/brief.go`, `docs/guide/how-merge-gate.md` |

Order notes:

- `internal/inspect/inspect.go` is edited by 899-B, 899-E, and 899-D. Each is in a different wave: B in wave 1, E in
  wave 2, D in wave 3.
- 899-D needs the probe (B) and the sandbox (C).
- 899-G changes the rules builders follow, so it lands after the code that makes them true (D, E).

## Why fewer than 10 lanes

The core of the Feature is one file, `internal/inspect/inspect.go`, and one rule chain (probe, sandbox, block). The
widest wave has two lanes. More lanes would edit `inspect.go` at once.

## Shared hot files

- `internal/inspect/inspect.go`: B, E, D in order. #897 (risk tier, review path) also edits `internal/inspect/`.
  Sequence the two Features so their `inspect.go` changes do not share a wave.
- `AGENTS.md`: 899-G. Many Features edit it; merge 899-G alone.
- `docs/guide/how-merge-gate.md`: 899-G, and #897's review path.

## Steps that need a person

| Step | Why | Source |
|---|---|---|
| 899-C | `.github/workflows/aayudha-merge-gate.yml` is in `[autonomy] always_person` and under `/.github/` in CODEOWNERS. | `.aayudha/config.toml:84`; `.github/CODEOWNERS` |
| 899-G | `/skills/` is a protected path. | `.github/CODEOWNERS` |
| Any `[inspect]` config key (an Expected path is `internal/config/defaults.toml`) | `inspect` is in `always_person_sections`; `defaults.toml` is in `always_person_config`. | `internal/config/defaults.toml:70`; `.aayudha/config.toml:102` |
| Declining a P0 or P1 | Never on an agent's own; a person decides. | `AGENTS.md`, Code review |
| Choosing the sandbox | Not stated in any source. | #899 |
