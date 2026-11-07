# Feature #977: aayu inbox, wave 4: the deck, phone pairing, the Behaviours view, and Ask why

- Issue: [#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977)
- Milestone: v0.4.0 (Release issue [#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906))
- Labels: `type:feature`, `status:planned`, `priority:P2`, `effort:M`, `risk:high`, `horizon:next` ([#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977))
- Parent: Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870), wave 4 of 4 of the inbox ([#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) body)

## Summary

On a phone or a small screen, the founder is to clear an area one card at a time in a deck whose swipes never ship.
He is to pair a phone on his tailnet with a one-time QR code, read every behaviour he approved in one view, ask why
on any shipped item, and ask Atri for counsel on any card
([#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) Summary).

**None of it is built.** Wave 4 sits on the web face of wave 3 ([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976)),
and that does not exist on dev at bc78349 (884d81b plus #1238): there is no `internal/cli/inboxweb/` directory. No PR into dev since v0.3.0
refers to these Stories (`_data/merge-refs.tsv`, `_data/open-prs.tsv`). The ledger has no `specs/977/` records.

## Status

| | Stories | Bugs |
|---|---|---|
| Delivered (merged into dev) | 0 | 0 |
| In review (open PR) | 0 | 0 |
| Pending (not started) | 5 | 0 |
| Total sub-issues | 5 | 0 |

Source: `_data/subs.tsv` (all 5 rows `OPEN`, `status:planned`), `_data/merge-refs.tsv`, `_data/open-prs.tsv`.

## What blocks it

- It depends on [#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) (0 of 13 Stories built),
  [#890](https://github.com/rahulnakmol/agentic-ayudha/issues/890) (whiteboard defense, milestone v0.4.1), and
  [#891](https://github.com/rahulnakmol/agentic-ayudha/issues/891) (behaviours) ([#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) Links).
  All are open and `status:planned` (GitHub, read 2026-11-07).
- Its Stories are made Ready only after the intent, spec, and UX design baselines are approved, the prototype states
  pass Hanu's review (D4), and it passes Approve to build ([#877](https://github.com/rahulnakmol/agentic-ayudha/issues/877))
  ([#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) Scope).

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Specification](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md): the planned phone and web screens, the most important file for this Feature
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
