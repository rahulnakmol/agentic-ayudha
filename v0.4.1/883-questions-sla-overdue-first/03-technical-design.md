# #883 Technical design

No tech baseline exists (`specs/883` is absent from the ledger). Everything under "Planned" is planned, not built.

## What exists on dev that this builds on

All paths are at `dev` [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b).

| Part | Where | What it does now |
|---|---|---|
| SLA policy | `decide.Policy`, `decide.NewPolicy` (`internal/decide/decide.go:100-122`) | Reads `[decisions]` and `[time]`; in solo mode uses the shorter provisional window. |
| Evaluate | `Policy.Evaluate` (`internal/decide/decide.go:166-192`), `decide.Step` (`:153-162`) | Sets `Lapsed` at the SLA, `Overdue` past the provisional time, and `Apply` only for a reversible, Decision-ready Decision. |
| Sweep | `decideSweep` (`internal/cli/decide.go:160-`) | For each open Decision: pages with key `decision-sla:<repo>#<n>`, or `decision-overdue:<repo>#<n>` when overdue (`:214-218`). |
| Config | `[decisions] sla_hours = 24`, `provisional_after_hours = 48`, `solo_provisional_after_hours = 24` (`internal/config/defaults.toml:199-203`); fields in `internal/config/config.go:348-350` | The two clocks #862 reuses. |
| Business hours | `[time] business_hours = "Mon-Fri 09:00-18:00"` (`internal/config/defaults.toml:214`) | Clock for both deadlines. |
| Loop | `loops/decisions.toml` (`command = ["decide"]`) | Runs the sweep on a schedule. |
| Mission asks | `MissionAskMarker`, `MissionAskDoneMark` (`internal/cli/missionask.go`); `InboxMission` (`internal/cli/inbox.go:50`) | A mission posts a step only a person takes as one comment; the inbox shows it as one card. The nearest thing to a question on `dev`. |
| Inbox Decision card | `InboxDecision` (`internal/cli/inbox.go:46`), `InboxItem.Provisional` | Shows an open Decision, or a provisional one to ratify or revert. |
| Mission clock | Story #1015, PR [#1091](https://github.com/rahulnakmol/agentic-ayudha/issues/1091), commit [2c6c6b0](https://github.com/rahulnakmol/agentic-ayudha/commit/2c6c6b04) | A 24 h card and a 48 h hold for missions, with a person's reason. A similar pattern, not this Feature. |

Not on `dev`: an `ask` verb (no `Name: "ask"` in `internal/verbs/registry.go`; `aayu ask --help` prints the root
help), a question record, an Overdue area or banner, and `internal/needs` (#873). Bug
[#859](https://github.com/rahulnakmol/agentic-ayudha/issues/859), "Deferred pages are never sent, and their key is
never paged again", is open and `status:planned` in v0.4.0.

## Layers touched

From `LAYERS.md`: L02 (`internal/cli/decide.go`, `inbox.go`), L03 (`internal/decide/**`, `loops/**`), L07
(`internal/config/**`). #883's Expected path `internal/cli/ask.go` does not exist yet; it is #880's.

## Planned, not built

1. **Question clock (P1).** Read open questions from #880's record and give each the same deadlines as a Decision,
   through `decide.Policy` (#862: "Both clocks use `[decisions] sla_hours` and `provisional_after_hours`").
2. **Sweep and page (P2).** `decideSweep` walks questions, pages once at 24 h with its own key, and sends deferred
   pages (#862 Evaluation, Bug #859).
3. **Provisional answer (P3).** At 48 h a two-way question takes the seat's recommendation, labelled provisional;
   the asking session is told; Decide shows it to ratify or revert. A one-way question only re-pages, as
   `Evaluate` does for a non-reversible Decision.
4. **Overdue area (P4).** The inbox's Overdue area after Now and a banner on every view; a snooze past due asks for a
   reason (shared with #884's past-due slice).

## Key decisions

| Decision | State | Effect |
|---|---|---|
| [#862](https://github.com/rahulnakmol/agentic-ayudha/issues/862) | accepted 2026-10-23 | The Decision SLA plus an Overdue area for agent questions. |
