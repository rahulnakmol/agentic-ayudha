# Sequencing: #911 loops run under Kala

The slices are the proposed ones in [05-implementation-plan.md](05-implementation-plan.md) (proposed, not filed).

## Before any build

1. A person confirms the owner key: `[hosts] repos` on `dev` or `[repos."<owner>/<repo>"] tool` in the issue.
2. File the Stories (`aayu slice 911`), then a person approves the intent, spec, and technical baselines and gives
   Approve to build ([af81cb48](https://github.com/rahulnakmol/agentic-ayudha/commit/af81cb48), #1018;
   [45a18e74](https://github.com/rahulnakmol/agentic-ayudha/commit/45a18e74), #1024).
3. Check per host whether it can hold a parent thread with subthreads (#911 Scope). This is research with each tool,
   not code.

## Waves

| Wave | Lane | Slice | Files or packages it edits |
|---|---|---|---|
| 1 | 1 | 911-A `aayu loop --due` | `internal/loops/due.go` (new), `internal/cli/loops.go`, `internal/verbs/registry.go`, generated docs |
| 1 | 2 | 911-B `[loops] check_minutes` | `internal/config/config.go`, `internal/config/defaults.toml`, `internal/config/gen` |
| 1 | 3 | 911-C new `active.json` record | `internal/loops/record.go`, `internal/doctor/**` |
| 2 | 1 | 911-D install creates Kala | `internal/cli/installloops.go` |
| 2 | 2 | 911-E `loop` skill | `skills/loop/` (new), `skills/mission/SKILL.md`, `evals/cases/loop/` (new, inferred) |
| 3 | 1 | 911-F Amp adapter | `internal/loops/amp.go` (new), `loops/amp/` |
| 3 | 2 | 911-G Claude Code adapter | `internal/loops/claude.go` (new), `loops/claude/` |
| 3 | 3 | 911-G Codex adapter | `internal/loops/codex.go` (new), `loops/codex/` |
| 3 | 4 | 911-G Cursor adapter | `internal/loops/cursor.go` (new), `loops/cursor/` |
| 4 | 1 | 911-I event loops wake the subthread | `internal/loops/loops.go` (`AmpSubscription`), `internal/loops/amp.go` |
| 4 | 2 | 911-J inbox Loops row and halt one loop | `internal/cli/inbox*.go`, `internal/cli/loops.go` |
| 5 | 1 | 911-H Copilot adapter | `internal/loops/copilot.go` (new), `internal/loops/loops.go` (`Hosts`), `loops/copilot/` |
| 5 | 2 | 911-H OpenCode adapter | `internal/loops/opencode.go` (new), `loops/opencode/` |

Order and merge notes:

- Wave 3 puts each adapter in its own new file so four lanes can run at once. Each adapter must register itself in
  `internal/loops/loops.go`. That registration is the one shared edit: merge the lanes in the order F, G-Claude,
  G-Codex, G-Cursor, and rebase each on the one before. The file names are a proposal, not an existing layout.
- Wave 4: 911-J waits for #895's Team view. 911-I edits `internal/loops/amp.go` after 911-F merges.
- Wave 5: 911-H waits for #917 (Copilot as a receiver) and #893 (OpenCode host ID). Both lanes edit `Hosts` in
  `internal/loops/loops.go`: merge Copilot first, then OpenCode.

## Why fewer than 10 lanes

The widest wave has four lanes. The base (911-A, 911-B, 911-C) must exist before any adapter, and the adapters are
the only part that splits wide. Two adapters (Copilot, OpenCode) wait for other Features.

## Shared hot files

- `internal/verbs/registry.go` (911-A adds a command): generated docs follow. One lane only.
- `internal/loops/loops.go`: 911-F to 911-I register here. Serial merges, as above.
- `internal/cli/loops.go`: 911-A in wave 1, 911-J in wave 4. Not in the same wave.
- `skills/mission/SKILL.md`: 911-E only. Kanha work under #889 may also edit it; check `_data/open-prs.tsv` first.

## Steps that need a person

| Step | Why | Source |
|---|---|---|
| 911-B | `internal/config/` is in `always_person`; `internal/config/defaults.toml` is in `always_person_config` and in `.github/CODEOWNERS`. | `.aayudha/config.toml:75-102`; `.github/CODEOWNERS` |
| 911-E | `/skills/` is a protected path. | `.github/CODEOWNERS` |
| Any change to `loops/build-ready.toml` | It is in `always_person`. | `.aayudha/config.toml` |
| Any change to `hosts/<host>/hooks.json` or `hosts/amp/aayudha.ts` | Protected paths. | `.github/CODEOWNERS` |
| Setting up Kala in a real tool | A person's own agent tool and sign-in. | #911 Scope |
