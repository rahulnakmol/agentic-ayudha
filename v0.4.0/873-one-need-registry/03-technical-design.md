# Technical design: one Need registry

Status: planned, not built. No tech baseline exists. The planned design is the #873 body.

## Layers touched

| Layer | Owner seat | Paths (from #873 Expected paths) |
|---|---|---|
| L02 Interaction / API boundary | Code Architect | `internal/cli/inbox.go`, `inboxtui.go`, `inboxaudit.go`, `inboxready.go`, `inboxwhy.go`, `inboxwork.go`, `scope.go`, `decide.go`, `build.go`, `delegate.go`, `halt.go`, `status.go`, `streak.go`, `inspect.go`, `inspect_app.go` (all under `internal/cli/`) |
| L03 Application / workflow | Engineering Manager | `internal/inspect/inspect.go` |
| L05 Intelligence | Data & AI Architect | `internal/models/gateway.go` |
| L06 Contracts / integration | Code Architect | `internal/notify/**` |
| none today | — | `internal/needs/**`: a new package that matches no layer glob (`internal/verbs/registry.go:363`). A slice with paths in no layer is refused (`internal/planning/slice.go:248`), so P1 adds the glob. |

## What exists on dev (bc78349)

| Part | Where | Today |
|---|---|---|
| Kind constants | `internal/cli/inbox.go:46-63` | 18 kinds: decision, scope, baseline, approve, mission, mission-start, locked, ready, not-ready, human-review, capped, unshipped, fixed, replaced, audit, mission-time, evening, overnight. `overnight` came with Story #1178 ([bc78349](https://github.com/rahulnakmol/agentic-ayudha/commit/bc78349)); 884d81b had 17. |
| Order | `internal/cli/inbox.go:69` `inboxOrder` | One fixed order: "the morning's decisions, baselines, Approve to build, and mission start first, then the other cards (Story #1175)". |
| Kind metadata | `internal/cli/inbox.go:83` `inboxKind`, `:94` `inboxKinds` | Icon, Word, Seat, Housekeeping, Line, Explain, Terms. It is "the one source of the kind's icon and word, the seat whose twin counsels on it, whether it is housekeeping …" (comment above `inboxKind`). This is the nearest thing to a registry. It has no area, door, due rule, wake rules, page key, or question. |
| Page | `internal/notify/notify.go:106` `Sender.Page`; `notify.Page` has `Key`, `Subject`, `Body`, `Urgent` | A non-urgent page outside business hours is deferred (`:123`). `Sender.Flush` (`:136`) has no caller outside a test (`internal/models/models_test.go:385`), which is Bug #859. |
| `internal/needs` | none | Does not exist. |

### Page call sites (11)

| File and line | Key |
|---|---|
| `internal/cli/build.go:1213` | `build:<repo>#<n>` |
| `internal/cli/delegate.go:978` | `delegate:<repo>#<n>` |
| `internal/cli/decide.go:214` | `decision-sla:<repo>#<n>` |
| `internal/cli/halt.go:297` | `halt:<repo>` (urgent) |
| `internal/cli/halt.go:413` | `resume:<repo>` (urgent) |
| `internal/cli/inspect.go:685` | `inspect:<repo>#<n>` |
| `internal/cli/inspect_app.go:153` | `inspect-cap:<repo>#<n>` |
| `internal/cli/inspect_app.go:353` | `inspect:<repo>#<n>` |
| `internal/cli/status.go:294` | `status-digest:<repo>` |
| `internal/cli/streak.go:160` | `autonomy-corrections:<repo>` |
| `internal/models/gateway.go:208` | `budget:<scope>` |

Source: `rg -n 'notify.Page\{' internal -g '!*_test.go'`. The issue said ten; the count is now eleven.

## Planned design (from #873)

1. Move first, with no change in behaviour, then add the fields (#873 Scope). Planned, not built.
2. A Need kind: area, seat, door, due rule, wake rules, actions, page key, question, next action (#873 Summary).
3. Areas: Now, Overdue, Ship, Answer, Decide, Ready, Housekeeping (#873 Scope, Q13). Decision
   [#862](https://github.com/rahulnakmol/agentic-ayudha/issues/862) (accepted 2026-10-23) places Overdue "right after
   Now" for questions.
4. A lint fails on a page with no kind (#873 Scope). Inferred: a Go test that scans `notify.Page{` literals, or a
   `Kind` field on `notify.Page` that `Sender.Page` refuses when empty.
5. The precision fields of V9 (#873 Scope). Unverified: V9 is an ID in the inbox design, which is not in this
   repository.

## Data and ledger records

No ledger record is named. The notify state stays local (`.aayudha/state/notify.json`, #859 Assumptions).

## Decisions

- [#862](https://github.com/rahulnakmol/agentic-ayudha/issues/862): the Decision SLA plus an Overdue area for
  questions (accepted 2026-10-23).
- Bug [#859](https://github.com/rahulnakmol/agentic-ayudha/issues/859) defers one question to "the engine design
  (Need registry, S4)": "whether a page that waited more than a day should still be sent, or say how old it is"
  (#859 Open questions).
