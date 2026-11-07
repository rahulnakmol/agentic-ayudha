# Sequencing: #900 the factory learns from its own records

The slices are the proposed ones in [05-implementation-plan.md](05-implementation-plan.md) (proposed, not filed).

## Before any build

1. File the Stories (`aayu slice 900`), with Decision #916's rules in them. A person approves the baselines and gives
   Approve to build ([af81cb48](https://github.com/rahulnakmol/agentic-ayudha/commit/af81cb48), #1018;
   [45a18e74](https://github.com/rahulnakmol/agentic-ayudha/commit/45a18e74), #1024).
2. A person defines "failure class" for CI and build failures, and names the Build board's surface.
3. #880 (`aayu ask`) must land before 900-B. #897 (merge records) must land, and run 4 weeks, before 900-F.

## Waves

| Wave | Lane | Slice | Files or packages it edits |
|---|---|---|---|
| 1 | 1 | 900-A run outcome | `internal/cli/build.go`, `internal/cli/buildnext.go`, `internal/cli/delegate.go`, `internal/ledger/**` (a new record file) |
| 1 | 2 | 900-E evolve refuses shipped skills | `internal/cli/evolve.go`, `internal/evals/evolve.go`, `internal/config/defaults.toml`, `internal/config/config.go`, `internal/config/gen` |
| 2 | 1 | 900-C three-times rule | `internal/cli/retro.go`, `internal/cli/fixlayer.go`, `internal/ledger/**` (reader of 900-A's record) |
| 2 | 2 | 900-B blocked opens an ask (after #880) | `internal/cli/build.go` |
| 3 | 1 | 900-D evolve trigger | `internal/cli/evolve.go`, `loops/evolve.toml` |
| 4 | 1 | 900-F routing evidence (after #897 plus 4 weeks) | `internal/models/**`, `internal/cli/models.go`, `internal/cli/streak.go`, `internal/cli/inspect_app.go` |

Order notes:

- 900-B and 900-A both edit `internal/cli/build.go`, so 900-B is in wave 2.
- 900-D and 900-E both edit `internal/cli/evolve.go`, so 900-D is in wave 3.
- 900-F is last, as the issue says.

## Why fewer than 10 lanes

The Feature has six slices, in a chain: outcome, then the counter, then the trigger, then routing. Two of them wait for
other Features (#880, #897). The widest wave has two lanes.

## Shared hot files

- `internal/cli/build.go`: 900-A, then 900-B. Other Features also edit it (#880's ask cites
  `internal/cli/build.go:1110-1135`). Check `_data/open-prs.tsv` before each wave.
- `internal/cli/evolve.go`: 900-E, then 900-D.
- `internal/config/defaults.toml` and `internal/config/gen`: 900-E. Generated `CONSTRAINTS.md` may change with it.

## Steps that need a person

| Step | Why | Source |
|---|---|---|
| 900-E | `internal/config/` is in `always_person`; `internal/config/defaults.toml` is in `always_person_config` and `.github/CODEOWNERS`. | `.aayudha/config.toml:75-102` |
| Every evolve PR | evolve never merges; a person reviews it. | `internal/cli/evolve.go`, `runEvolve` comment |
| 900-F routing changes | A lane default change touches `[models]`, which is in `always_person_sections`, when made in config. | `internal/config/defaults.toml:70` |
| Definitions | "Failure class" and the Build board need a person's choice. | this plan |
