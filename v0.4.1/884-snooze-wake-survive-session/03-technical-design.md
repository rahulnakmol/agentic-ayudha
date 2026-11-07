# #884 Technical design

No tech baseline exists (`specs/884` is absent from the ledger). Everything under "Planned" is planned, not built.

## What exists on dev that this builds on

All paths are at `dev` [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b).

| Part | Where | What it does now |
|---|---|---|
| Skip | `state map[string]string` (`internal/cli/inboxtui.go:442`); key `s` (`:1394-1398`); render "skipped for now" (`:2166-2167`) | Session-only skip. The finish line counts done, skipped, and left (`:288-302`). |
| Inbox items | `InboxItem` (`internal/cli/inbox.go:249-`), with `Updated` and `Head` | `Updated` is when the issue or PR last changed on GitHub; `Head` is the PR head the card was read at. A wake-on-change rule can compare these (inferred). |
| Inbox action record | `UpsertComment` with marker `InboxMarker`, action kind `github.comment.inbox` (`internal/cli/inbox.go:2009-2019`) | Records a done housekeeping step as a comment on the issue. |
| Ledger writer | `ledger.Writer.Write` and `Create` (`internal/ledger/ledger.go:112`, `:165`) | Commits records to the `ledger` branch. |
| JSONL on the ledger | `MissionClaimsPath` = `missions/<f>/claims.jsonl` (`internal/cli/missionnext.go:68`) | An existing append-style record a new `inbox/<login>.jsonl` can follow (inferred). |
| Business hours | `[time] business_hours = "Mon-Fri 09:00-18:00"` (`internal/config/defaults.toml:214`); `config.Config.Clock` used by `decide.NewPolicy` (`internal/decide/decide.go:112`) | Gives "09:00 tomorrow" and "Monday 09:00" per #939 criterion 3. |
| Counsel post | `aayu counsel --post` (`internal/verbs/registry.go`, entry `Name: "counsel"`; `internal/cli/counsel.go:92-93`, `:241-267`) | Posts a checked counsel on a PR or issue; needs `--draft` and `--pr` or `--issue`. |
| Login | `internal/cli/inboxlogin.go` | The person's login for the inbox (file present; its use is not checked here). |

Not on `dev`: an `inbox/` folder on the ledger, a snooze or wake record, a "With others" area, and `internal/needs`
(#873).

## Layers touched

From `LAYERS.md`: L02 (`internal/cli/inbox.go`, `inboxtui.go`, `inboxaudit.go`), L07 (`internal/config/**`,
`internal/ledger/**` if a path helper is added), L04 (`CONSTRAINTS.md`). `internal/needs/**` needs a layer glob first
(`TestEveryInternalPackageInALayer`, `internal/verbs/gen/main_test.go:20`).

## Planned, not built

1. **Record (P1).** `inbox/<login>.jsonl`: one line per snooze or skip, with the item key, until when, and the wake
   condition. The TUI's `s` writes it through the ledger writer, and the gather drops a card whose snooze holds.
2. **Wake (P2).** A card comes back at its time, or sooner when its head, checks, or comments change (#939 criterion
   2).
3. **Past due (P3).** A snooze of a question past due needs a reason and posts a comment (#884; Decision #862 option 1:
   "the seat is told"). Needs questions from #880 and the due time from #883.
4. **With others (P4).** Ask a person by mention, or Atri by `aayu counsel --post`; the card moves to "With others"
   and wakes on the answer.

## Data and ledger records

Planned: `inbox/<login>.jsonl` on the `ledger` branch (#884 Scope). Its fields are not in any source.

## Key decisions

- [#862](https://github.com/rahulnakmol/agentic-ayudha/issues/862), accepted 2026-10-23: "A snooze past the due time
  needs a one-line reason, and the seat is told."
- #884 links `depends-on` [#873](https://github.com/rahulnakmol/agentic-ayudha/issues/873).
