# UX design: one Need registry

Status: P1 (the move) changes no UX by design (#873 Scope). The later slices change how the inbox groups cards.

## The person today (bc78349)

From `aayu inbox --help`:

> Every human-only step waiting for you, one card at a time with the twin's counsel; Enter takes the recommendation
> and C clears the housekeeping. Refused for agents and needs a terminal, except --list.

- `--list`: "Read-only: print what waits as the same cards, with the counsel and how a person acts on them, as
  Markdown (or JSON with --json); writes nothing, needs no terminal, and an agent may run it."
- Each card shows an icon and a word, the plain line, and an explainer behind `?` (`inboxKind`,
  `internal/cli/inbox.go:83`). Housekeeping kinds are the ones C clears: `unshipped`, `fixed`, and `replaced` have `Housekeeping: true`, and
  `inboxTidy` is the Housekeeping card (`internal/cli/inbox.go:190-226`).
- Cards come in one fixed order (`inboxOrder`, `internal/cli/inbox.go:69`).

## Planned

- Cards grouped by area: Now, Overdue, Ship, Answer, Decide, Ready, Housekeeping (#873 Scope). Each card shows its
  question and next action (#873 Summary). Planned, not built.
- A page names its kind, so the page and the card agree (#873 title). The page text format is not specified.
- No mockup exists: no `specs/873/baselines/ux-*.json` on the ledger.

## The agent

`aayu inbox --list --json` is the agent's read. Planned: each item carries its Need kind and area. The JSON shape is
not specified in any source.
