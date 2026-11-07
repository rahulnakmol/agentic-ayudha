# #884 UX design

The surface is the `aayu inbox` terminal session. `inbox` is human-only and needs a terminal, except `--list`
(`aayu inbox --help`). The web screens are out of scope (#884).

## Today on dev

`aayu inbox --help` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b):

```text
Every human-only step waiting for you, one card at a time with the twin's counsel; Enter takes the recommendation and C clears the housekeeping. Refused for agents and needs a terminal, except --list.

Usage:
  aayu inbox [flags]

Flags:
  -h, --help   help for inbox
      --list   Read-only: print what waits as the same cards, with the counsel and how a person acts on them, as Markdown (or JSON with --json); writes nothing, needs no terminal, and an agent may run it.
```

In the TUI, the key help reads `s` = "skip for now" (`internal/cli/inboxtui.go:2587`). After `s` the status line is
"Skipped #`<n>` for now; j and k come back to it." (`:1397`), and the card shows "skipped for now" (`:2167`). The skip
ends with the session.

## Planned, not built

Story [#939](https://github.com/rahulnakmol/agentic-ayudha/issues/939) (under #975) gives the terminal design at
120x34:

- `s` on a Ship card: "Skip #825 until: 1 it changes · 2 09:00 tomorrow · 3 Monday 09:00" (criterion 1).
- After 3: "Skipped #825 until Monday 09:00. It comes back sooner if its head, checks, or comments change." (criterion
  2).
- `s` on an ask due in 21 h: "Snooze until: 1 in 2 h · 2 tomorrow 09:00 · 3 when #817 merges" and "It is due in 21 h.
  A snooze past the due time asks for a reason, and Shuka is told." (criterion 4).
- After 2: "Snoozed until 09:00 tomorrow. Shuka is told the answer comes by then." The card moves to Snoozed
  (criterion 5).
- `?` on an ask: "1 Ask Atri for counsel · 2 Ask @alice (posts the question to her) · 3 Ask Shuka for more evidence"
  (criterion 6).
- After 2: "Asked @alice on #812. The card moves to With others. It comes back when she answers or at 09:00."
  (criterion 7).

Issue #884 adds that the terminal, web, phone, and a second machine show the same snooze state.

## Agent-facing surface

An agent may read `aayu inbox --list`. Whether `--list` shows snoozed cards is not in any source.

## Accessibility

The global `--plain` flag removes colour, motion, emoji, and box drawing. The sources state no other requirement.
