# Goal and intent: #911 loops run under Kala

No intent baseline exists: the ledger has no `specs/911/` path (`origin/ledger`, 2026-11-07). The source is the Feature
issue [#911](https://github.com/rahulnakmol/agentic-ayudha/issues/911), checked against `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b).

## Goal

"A repository's loops run under Kala, one thread on every host. Each loop keeps one subthread, and each run of a loop
wakes that subthread again instead of starting a new thread. aayu decides which loops are due; Kala only wakes them."
([#911](https://github.com/rahulnakmol/agentic-ayudha/issues/911), Summary.)

Kala keeps the repository's loops; Kanha (#889) keeps a Feature's builds. "The two share one code path and have
different names so they are easy to tell apart (@rahulnakmol, 2026-10-22; decision Y10)" (issue text). Decision Y10 is
not in this repository. Unverified.

## Who it serves

The founder, and any person who sets up a repository's loops. The issue says the founder "sees 14 parent threads and a
new subthread for every run, with no one place that shows what the loops did" (#911, Problem).

## Problem, checked against the code

| Claim in #911 | What `dev` shows |
|---|---|
| `loops/amp/*.json` sets `"run_mode": "fresh"`. | True. `Loop.AmpSchedule` writes `"fresh"` ("run_mode fresh starts a new thread per firing", `internal/loops/loops.go:697-707`). `loops/amp/` holds 14 files. |
| `loops/active.json` records 14 Amp loop threads (2026-10-19). | On 884d81b the file records 13 Amp entries. `build-ready` has a render in `loops/amp/` but no entry. The file last changed in [3d988a97](https://github.com/rahulnakmol/agentic-ayudha/commit/3d988a97) (#710), before v0.3.0. |
| Claude Code routines, Codex scheduled tasks, and Cursor automations each start a separate session per run. | The renders exist: `loops/claude/routines.md`, `loops/codex/scheduled-tasks.md`, `loops/cursor/aayudha-loops.tf`. The per-run session behaviour is the hosts' own and is not checked here. |
| OpenCode has no loop adapter. | True. `loops.Hosts` is `amp, cursor, claude, codex, actions` (`internal/loops/loops.go:48`). Copilot has none either. |

## Success measures

The issue states no numeric measure. Its observable outcomes:

1. One Kala thread per repository, made by `aayu install --loops` (#911 Scope).
2. Each loop has one subthread, made on its first run and reused after (#911 Scope).
3. Kala never runs a loop that `aayu loop --due` did not list (#911 Scope).
4. The inbox Team view shows a "Loops" row with each loop's last run, result, and age (#911 Scope, through #895).

## Out of scope

From #911, Scope "Out": changing what each loop does; changing the cron times; GitHub Actions loops (`agent = false`),
which stay in Actions. In this repository those are `digest` and `verify-sweep` (`loops/README.md`).
