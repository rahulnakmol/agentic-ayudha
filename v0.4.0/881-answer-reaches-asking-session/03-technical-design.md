# Technical design: the answer reaches the asking session

Status: planned, not built. No tech baseline exists. The planned design is the #881 body.

## Layers touched

| Layer | Owner seat | Paths (from #881 Expected paths) |
|---|---|---|
| L02 Interaction / API boundary | Code Architect | `internal/cli/ask.go` (from #880), `internal/cli/install.go`, `internal/cli/inbox.go` |
| L03 Application / workflow | Engineering Manager | `internal/delegate/**`, `internal/verbs/registry.go` |
| L06 Contracts / integration | Code Architect | `internal/hosts/**`, `hosts/**` |
| L07 Operational data / state | Production Engineer | `internal/config/defaults.toml` |
| not in a layer glob | — | `skills/ask/SKILL.md` (new) |

## What exists on dev (884d81b)

| Part | Where | Today | Since |
|---|---|---|---|
| Adapter contract | `internal/delegate/receivers.go:104` `Adapter` | `Handoff`, `Status`, `FollowUp(ctx, st State, msg string) (Task, error)`. | before v0.3.0 |
| Four push channels | `CursorAPI.FollowUp` `:230`, `AmpCLI.FollowUp` `:290`, `ClaudeCloud.FollowUp` `:393`, `CodexCloud.FollowUp` `:492` | Each sends a text message to an existing task or thread. These are the four adapters #881 names. | before and after v0.3.0 (not checked per adapter) |
| Uses of `FollowUp` | `internal/cli/delegate.go:1146` (review fixes); `internal/cli/kanha.go:78` (Kanha brief to the drafting Amp thread); `internal/cli/kanha.go:169` and `internal/cli/missiontick.go:300` at bc78349 (`:282` at 884d81b) (Kanha events); `internal/cli/missionstop.go:488` (stop a story thread) | A push to a known session already works for missions. | Kanha events: [7a9f3d7](https://github.com/rahulnakmol/agentic-ayudha/commit/7a9f3d7e) (#1097) |
| Pull model | `internal/cli/kanha.go:156-157`; `internal/cli/kanhalocal.go` | A local Kanha's events are `recorded; the local Kanha on <machine> takes it at its next check` (`aayu mission wake`, every `[mission] check_minutes`). This is close to the planned `pull-pending` receipt (inferred). | Story #1012 |
| Delivery record | `internal/delegate/state.go:85` `FollowUp`, `:94` `Event` | A follow-up records time, head, cycle, findings, and checks. A Kanha event records its text and "sent" or "not sent: …" in `Detail`. No receipt states. | — |
| `--wait` | none | No `aayu ask` (see #880). | — |

## Planned design (from #881)

1. A receipt with the states sent, pull-pending, read, failed (#881 Scope). Planned, not built. Where the receipt is
   stored is not named; inferred: next to the answer record `asks/<item>/<id>-answer.json` (#880 Scope).
2. A push "through the existing Amp, Claude cloud, Codex cloud, and Cursor adapters" (#881 Scope). Inferred: the
   session comes from #871's source on the ask record.
3. `aayu ask --wait` for a local session (#881 Scope). How it waits (poll, bound) is not named.
4. "read": no source says how aayu learns that a session read the answer. Unverified.
5. Config: `internal/config/defaults.toml` is listed; no key is named. Unverified.

## Decisions

No Decision is linked from #881.
