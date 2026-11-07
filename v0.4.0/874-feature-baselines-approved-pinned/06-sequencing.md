# Sequencing: Feature #874

## What is left

Two Stories: [#1028](https://github.com/rahulnakmol/agentic-ayudha/issues/1028) and
[#1106](https://github.com/rahulnakmol/agentic-ayudha/issues/1106). #1106 depends on #1028 and on Feature
[#883](https://github.com/rahulnakmol/agentic-ayudha/issues/883) (#1106 Links). Both edit
`internal/planning/plan.go` and `internal/cli/plan.go` (#1028 Notes; #1106 Notes). So this Feature has at most one
lane at a time. Ten lanes do not make sense here: two Stories in one dependency chain, on the same two files.

## Before any lane starts (a person)

1. Start the mission of #874. `missions/874/plan.json` is approved (approval 1) and has no `started_by`. Start is a
   person's step: `aayu mission start 874` is refused for an agent lane (`mission` entry in
   `internal/verbs/registry.go`).
2. Arjun makes #1028 Ready. Approval 1 covers it at its Completion Goal hash `5899aaa0…`
   (`specs/874/approved.json`). If its Completion Goal, effort, or title changes first, `storyCovered` in
   `internal/cli/readycover.go` refuses it, and `--ack` is not built
   ([#1082](https://github.com/rahulnakmol/agentic-ayudha/issues/1082)). Then the only route is a new spec
   baseline and a new Approve to build, both a person's steps.

## Waves

| Wave | Lane | Work | Files or packages | Waits for |
|---|---|---|---|---|
| 1 | A | #1028 risk register | `internal/planning/plan.go` (`CheckPlan`, plan record), `internal/cli/plan.go` (label write, result line), plan eval schema and `skills/plan/SKILL.md` if the draft shape changes (inferred) | mission Start; #1028 Ready |
| 2 | A | #1106 failure modes and open questions | `internal/planning/plan.go`, `internal/cli/plan.go`, `internal/cli/ready.go` (`--check` line, Ready refusal), `internal/cli/ask.go` (if the ask is filed from plan, #1106 Notes), `internal/config/config.go` and `defaults.toml` (`[ready] dor_from`) | #1028 merged; Feature #883 delivered (v0.4.1); the `aayu ask` verb of Feature #880 (not on `dev`) |

## Cross-Feature lanes that touch the same files

- #1028 unblocks [#1029](https://github.com/rahulnakmol/agentic-ayudha/issues/1029) (Feature #877), which also
  needs #995 (merged) and #1083 (#1029 Links). Run #1028 early. It shares no file with #1082 or #1086 (Feature
  #877) or #919 (Feature #875), so those can run in the same wave.
- #1106 edits `internal/cli/ready.go`. #1082 and #1083 (Feature #877) and #921 (Feature #875) edit it too. Merge
  #1106 after them, or rebase it on them.

## Hot files that force serialized merges

- `internal/planning/plan.go`, `internal/cli/plan.go`: both Stories.
- `internal/config/defaults.toml`, `internal/config/config.go`, `internal/config/gen`, and the generated
  `CONSTRAINTS.md`: #1106 adds `[ready] dor_from`. A new key in a rendered section needs a row in
  `internal/config/gen` (AGENTS.md, Generated files).
- `internal/verbs/registry.go` and its generated files: only if a Story adds a flag. The issue bodies name none.

## Steps that need a person

- Mission Start (above), and any new Approve to build or acknowledgment.
- #1106 changes `internal/config/defaults.toml`. It is in `[autonomy] always_person_config`
  (`.aayudha/config.toml`) and in `.github/CODEOWNERS`, and `internal/config/` is in `[autonomy] always_person`.
  So a person ships it, and `protected-paths` needs the founder's recorded bypass (AGENTS.md).
- A change to `skills/plan/SKILL.md` is a CODEOWNERS path (`/skills/`): the same bypass.
- PR [#1282](https://github.com/rahulnakmol/agentic-ayudha/issues/1282) (draft, Story
  [#1281](https://github.com/rahulnakmol/agentic-ayudha/issues/1281), Decision
  [#1255](https://github.com/rahulnakmol/agentic-ayudha/issues/1255)) would leave only the policy keys with a
  person. It was closed unmerged on 2026-11-07 when the mission of #1170 ended; branch
  `feat/1281-auto-merge-on-proof` at `6f2aae7` is kept (`_data/closed-prs.tsv`). So today's narrow set applies.
