# Sequencing: Feature #876

## Shape of the work

Three Stories in a chain: #1114 → #1115 → #1116 (Links sections; `specs/876/design.json` story order). #1114
needs nothing from Feature #875, because its tests seed review records on the ledger (#1114 Assumptions).
So Feature #876 can start before #875 is built. Ten lanes do not make sense here: three small Stories, each needing the one
before it.

## Before any lane starts (a person)

1. Approve #876 to build: `aayu ready 876` in the founder's terminal, with the device code (`Env.laneRefusal`
   refuses an agent lane). No `specs/876/approved.json` exists today.
2. Arjun makes #1114 Ready under that approval (`storyCovered` in `internal/cli/readycover.go`).

## Waves

| Wave | Lane | Story | Files or packages | Waits for |
|---|---|---|---|---|
| 1 | A | #1114 critique state, open-gap refusal, `--findings` | `internal/workitems/critique/` (new), one wiring line in `internal/cli/readyapprove.go` or `internal/cli/env.go`, `internal/cli/grill.go` (`--findings`), `internal/verbs/registry.go` (`--findings` flag on `grill`) | Approve to build of #876 |
| 2 | A | #1115 `aayu critique --resolve` | `internal/verbs/registry.go` (new `critique` verb), `internal/cli/critique.go` (new), `internal/workitems/critique/` (read `resolutions.jsonl`), `internal/config/config.go`, `defaults.toml`, `gen/main.go` (`reason_max_chars`); generated `CONSTRAINTS.md`, MCP schema, skill frontmatter | #1114 |
| 3 | A | #1116 grill skill section and eval case | `skills/grill/SKILL.md`, `evals/cases/grill/findings-walk/`, `evals/eval.toml` | #1114, #1115 |

## Cross-Feature conflicts

- `internal/verbs/registry.go` and the `critique` verb: #1115 creates it with `--resolve` only; #920 (Feature
  #875) adds the review flags and `skills/critique/SKILL.md` later (#1115 Assumptions). Put #1115 and #920 in
  different waves. Whichever merges second rebases and runs `go generate ./...`.
- `internal/config/` `[critique]` section: #1115 adds `reason_max_chars`; #919 (Feature #875) adds five keys.
  Serialize them.
- `skills/grill/SKILL.md`: Bug [#1086](https://github.com/rahulnakmol/agentic-ayudha/issues/1086) (Feature #877)
  also changes it (the parent-Feature rule, around lines 180-236 and 383 today). #1116 adds a new section. Merge
  #1086 first or rebase; both run the grill evals.
- `internal/cli/grill.go`: #1114 adds `--findings`; #1086 adds the Story parent refusal near the parent check
  (`internal/cli/grill.go`, the `d.Parent` block). Put them in different waves or name the order.
- `internal/cli/readyapprove.go`: #1114's wiring line; #1083 and #1029 (Feature #877) change the same file.

## Hot files that force serialized merges

- `internal/verbs/registry.go` and its generated files (all three waves may touch it).
- `internal/config/defaults.toml`, `CONSTRAINTS.md` (#1115).
- `evals/eval.toml`: a bar may only rise, and a lowered bar fails the `build` job's ratchet step (AGENTS.md,
  Coverage ratchet).

## Steps that need a person

- Approve to build of #876 (above). The dismissals that #1115 records are also a person's step at use time.
- #1115 changes `internal/config/defaults.toml` (`[autonomy] always_person_config` and `.github/CODEOWNERS`; #1115
  Assumptions say "a person ships this story").
- #1116 changes `skills/` (a CODEOWNERS path), so "a person ships this story" (#1116 Assumptions), with the
  founder's recorded bypass for `protected-paths`.
