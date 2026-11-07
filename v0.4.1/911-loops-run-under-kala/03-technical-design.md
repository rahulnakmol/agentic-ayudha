# Technical design: #911 loops run under Kala

No technical baseline exists (no `specs/911/` on `origin/ledger`). Part 1 is what `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b) has. Part 2 is the design that
[#911](https://github.com/rahulnakmol/agentic-ayudha/issues/911) describes. Part 2 is **planned, not built**.

## Layers touched

| Layer | Paths (Expected paths of #911, plus inferred ones marked) |
|---|---|
| L02 Interaction / API boundary | `internal/cli/installloops.go`, `internal/cli/loops.go`; inferred: `internal/cli/inbox*.go` for the Loops row |
| L03 Application / workflow | `internal/loops/`, `loops/`, `skills/loop/` (new), `skills/mission/` |
| L06 Contracts / integration | `hosts/` |
| L07 Operational data / state | inferred: `internal/config/` for `[loops] check_minutes` |

## Part 1: what exists on dev

| Building block | Where | Notes |
|---|---|---|
| Loop definitions | `loops/*.toml` (16 files; `digest` and `verify-sweep` are `agent = false`), `internal/loops/loops.go` (`Loop`, `Parse`) | One file per loop: command, cron or event, prompt. |
| Host renders | `internal/loops/loops.go` (`AmpSchedule`, `AmpSubscription`, the Cursor Terraform render), `loops/amp/`, `loops/claude/`, `loops/codex/`, `loops/cursor/` | Amp: `run_mode` `fresh`. Event loop on Amp: a GitHub subscription with destination `new-orb` (`AmpSubscription`). |
| Loop hosts | `internal/loops/loops.go:48` (`Hosts`) | `amp, cursor, claude, codex, actions`. No Copilot, no OpenCode. |
| Record | `internal/loops/record.go` (`RecordFile` = `loops/active.json`, `Record` = host → loop → `Entry{Digest, ID, At}`, `States`) | One entry per loop per host. |
| Setup | `internal/cli/installloops.go` (`runInstallLoops`, `loopsReport`) | The person picks one tool from those `agentenv.Detect` finds; Amp and Cursor with `--apply`, a ready prompt per loop for Claude Code and Codex. |
| Guard | `internal/cli/loops.go` (`LoopHookPrefix` = `loop:`, `runLoopGuard`) | `aayu hook loop:<name>`: exit 4 when halted, 6 when the budget breaker paused agent lanes; says whether this run is the scheduled one. |
| Person's tool per repository | `internal/config/config.go:74` (`Hosts.Repos`), `RepoTool`, `RepoTools` | `[hosts] repos` = `owner/name=tool`, user config only. From [26537bd6](https://github.com/rahulnakmol/agentic-ayudha/commit/26537bd6) (#1194). `RepoTools` lists the registry hosts, so OpenCode is not allowed yet. |
| Coordinator hand-off | `internal/delegate/receivers.go:20-33` (`Handoff.Scope`, `Handoff.Key`), `internal/ready/approve.go:41-57` (`Parent`) | Comments: "a coordinator's (a Feature's Kanha, Story #1004; Kala, #911) names its scope (feature or repo)". From [93255510](https://github.com/rahulnakmol/agentic-ayudha/commit/93255510) (#1092). |
| Kanha protocol | `skills/mission/SKILL.md`; [941f9ae5](https://github.com/rahulnakmol/agentic-ayudha/commit/941f9ae5) (#1095) | The protocol section Kala is to share. |
| Work list | `aayu mission next --json` emits `aayudha.work/v1`: `{schema, scope {kind, id}, items [{key, action, reason, subthread}], events [{at, story, event}]}` (`internal/verbs/registry.go:873`; `internal/cli/missionnext.go`) | `scope.kind` already allows a kind other than feature. Inferred from the schema shape. |
| Cursor of events | `events_cursor` in `missions/<f>/plan.json` (`internal/verbs/registry.go:873`) | The pattern for FR12. |
| Local wake | `aayu mission wake` (`internal/cli/missionwake.go`), [80af4e1a](https://github.com/rahulnakmol/agentic-ayudha/commit/80af4e1a) (#1125) | Stateless wake for a local Kanha every `[mission] check_minutes` (default 5, `internal/config/defaults.toml:95`). |
| Local Kanha sessions | `internal/delegate/local.go` (`KanhaStart`, `KanhaResume`, `KanhaSession`) | Start and resume one session for Codex, Copilot, OpenCode, and Cursor. The pattern for "one session per loop, resumed". |
| Safety tick | [e43876ec](https://github.com/rahulnakmol/agentic-ayudha/commit/e43876ec) (#1103) | Hourly tick that wakes a Kanha that missed an event. |

## Part 2: planned design (planned, not built)

1. **`aayu loop --due`** (new plumbing command, common English). It reads `loops/*.toml`, applies the guard logic of
   `runLoopGuard` to each, and prints the due list as `aayudha.work/v1` with `scope {kind: repo}`. No model. (#911
   Scope, FR6 and FR10.)
2. **`[loops] check_minutes`**, default 15. Kala's one schedule. (#911 Scope.) It is a new key in a config section; a
   rendered section needs a row in `internal/config/gen` (`AGENTS.md`, Generated files).
3. **Record.** `loops/active.json` changes from host → loop → entry to one Kala entry (owner, tool, thread) plus one
   subthread per loop, and a cursor (#911 Scope, FR12). The old shape needs a migration path. Inferred.
4. **Install.** `aayu install --loops` creates Kala in the owner's tool and moves the loops on a tool switch (#911
   Scope).
5. **Shared code path with Kanha.** Use `delegate.Handoff` with `Scope = "repo"` and `ready.Parent` (already shaped
   for it), and the `KanhaStart` / `KanhaResume` forms for hosts with one resumed session (#911 Summary, Y10).
6. **Skill.** A new `skills/loop/` that shares one protocol section with `skills/mission/` (#911 Scope).
7. **Adapters.** Amp, Claude Code, Codex, Cursor first. Copilot after #917 and OpenCode after #893 (#911 Links).
8. **Inbox.** A "Loops" row in the Team view, through #895 (#911 Scope). The evidence does not show a Team view in
   `internal/cli/inbox*.go` on 884d81b.

## Contracts

- `aayudha.work/v1`: shared between `aayu mission next --json` and `aayu loop --due` (#911 Scope).
- `loops/active.json`: read by `aayu doctor` today (`internal/loops/record.go`, `RecordFile` comment).

## Decisions

- Y10, the names Kala and Kanha (#911 Summary). Not in this repository. Unverified.
- R9 (owner's tool) and J4 (cursor and config, not prompt) are cited by the issue. Not in this repository. Unverified.
