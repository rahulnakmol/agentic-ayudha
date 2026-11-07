# Sequencing: Feature #875

## Shape of the work

The five Stories form a chain: #919 → #920 → #921 → #922 and #923 (#923 also needs #920). The depends-on links
are in each Story's Links section and in `specs/875/baselines/spec-v1.json`. So at most two lanes run at once, in
the last wave. Ten lanes do not make sense here: the Feature is five small Stories in one chain, and four of them
add flags to the same registry entry.

## Before any lane starts (a person)

1. Approve #875 to build: `aayu ready 875` in the founder's own terminal, with the device code. It is refused for
   an agent lane (`Env.laneRefusal` in `internal/cli/readyapprove.go`). With no reviews recorded yet, the
   critique stub lets it open (`Env.critique` returns an empty `ready.Critique`).
2. Then Arjun makes each Story Ready in order; `storyCovered` (`internal/cli/readycover.go`) admits each one that
   the approval lists at its current Completion Goal, effort, and title.

## Waves

| Wave | Lane | Story | Files or packages | Waits for |
|---|---|---|---|---|
| 1 | A | #919 reviewer table, finding type, `[critique]` keys | `internal/verbs/` (new table file and lint), `internal/workitems/critique/`, `internal/config/config.go`, `defaults.toml`, `gen/main.go`, `schema.json`; generated `docs/RACI.md`, `CONSTRAINTS.md` | Approve to build of #875 |
| 2 | A | #920 `aayu critique` records a review | `internal/verbs/registry.go` (new verb), `internal/cli/critique.go`, `skills/critique/SKILL.md`, generated `internal/mcp/schemas/critique.json` and skill frontmatter | #919 |
| 3 | A | #921 approval starts the reviews | `internal/cli/ready.go` (after `partRun.approve`), `internal/delegate/` (brief, adapters), `internal/verbs/registry.go` (`--cloud` on `ready`) | #920 |
| 4 | A | #922 sweep, status, loop | `internal/cli/critique_sweep.go` (suggested new file), `loops/baseline-reviews.toml`, rendered `loops/amp`, `loops/cursor`, `loops/claude`, `loops/codex`, registry flags `--sweep`, `--status` | #921 |
| 4 | B | #923 local session | `internal/cli/critique_local.go` (suggested new file), `internal/delegate/local.go`, registry flag `--session` | #920, #921 |

Lanes 4A and 4B both add flags to the `critique` entry in `internal/verbs/registry.go` and both regenerate its
schema and skill frontmatter. Merge #923 first, because it records the review that #922's sweep re-dispatches.
Then rebase #922 and run `go generate ./...` again. The suggested file names are mine, to keep the two lanes off
one file; the Stories name only `internal/cli/critique.go`.

## Cross-Feature conflicts

- Feature #876's [#1115](https://github.com/rahulnakmol/agentic-ayudha/issues/1115) also creates the `critique`
  verb, "on the CLI only, with --resolve only; #920 adds its review flags and its skill later" (#1115
  Assumptions). Whichever of #920 and #1115 merges first creates the registry entry and `internal/cli/critique.go`;
  the other rebases on it. Do not run them in the same wave.
- #921 edits `internal/cli/ready.go`, as do #1082 and #1083 (Feature #877) and #1106 (Feature #874). Name a merge
  order when they overlap.
- #919 and #1115 both add `[critique]` keys (#1115 adds `reason_max_chars`) in `internal/config/`. Serialize them.
- #1114 (Feature #876) needs nothing from #875: its tests seed review records (#1114 Assumptions). It can run in
  wave 1 beside #919, on other files.

## Hot files that force serialized merges

- `internal/verbs/registry.go` and everything generated from it: `docs/README.md`, `LAYERS.md`, `docs/RACI.md`,
  `skills/*/SKILL.md` frontmatter, `internal/mcp/schemas/*.json` (AGENTS.md, Generated files). Waves 1 to 4 all
  touch it.
- `internal/config/defaults.toml` and `CONSTRAINTS.md`: #919.
- `internal/cli/critique.go`: #920, #922, #923 (and #1115).

## Steps that need a person

- Approve to build of #875 (above).
- #919 changes `internal/config/defaults.toml`: it is in `[autonomy] always_person_config` and
  `.github/CODEOWNERS`, and `internal/config/` is in `[autonomy] always_person` (`.aayudha/config.toml`). A person
  ships it and the founder's recorded bypass clears `protected-paths` (#919 Assumptions; AGENTS.md).
- #920 adds `skills/critique/SKILL.md`, a CODEOWNERS path: the founder's bypass for `protected-paths` (#920
  Assumptions).
- #922 adds a loop. Only `loops/build-ready.toml` is in `always_person`, so the new loop file is not, as the
  config reads today.
