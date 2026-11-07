# Feature #976: aayu inbox, wave 3: the local web command center, with the same words as the terminal

- Issue: [#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976)
- Milestone: v0.4.0 (Release issue [#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906))
- Labels: `type:feature`, `status:planned`, `priority:P2`, `effort:L`, `risk:high`, `horizon:next` ([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976))
- Parent: Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870), wave 3 of 4 of the inbox ([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) body)

## Summary

`aayu inbox --web` is to open the same inbox in a browser: on this computer, over an SSH tunnel, or through an Amp
portal. Every card and state is to show with the same words as the terminal, and stay live. The plan adds a command
center frame, a PR viewer, lineage, browse by release and workstream, themes, a command bar, empty states, a web
Mission page, and WCAG 2.2 AA ([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Summary).

**None of it is built.** On dev at bc78349 (884d81b plus #1238) there is no `internal/cli/inboxweb/` directory, and the `inbox` verb has
only the `--list` flag (`internal/verbs/registry.go`, `plumb("inbox", …)`). No PR into dev since v0.3.0 refers to
these Stories (`_data/merge-refs.tsv`, `_data/open-prs.tsv`). The ledger has no `specs/976/` records: no intent,
spec, tech, or UX design baseline (`git ls-tree origin/ledger specs/976` is empty).

## Status

| | Stories | Bugs |
|---|---|---|
| Delivered (merged into dev) | 0 | 0 |
| In review (open PR) | 0 | 0 |
| Pending (not started) | 13 | 0 |
| Total sub-issues | 13 | 0 |

Source: `_data/subs.tsv` (all 13 rows `OPEN`, `status:planned`), `_data/merge-refs.tsv`, `_data/open-prs.tsv`.

## What blocks it

- The Feature depends on [#975](https://github.com/rahulnakmol/agentic-ayudha/issues/975),
  [#885](https://github.com/rahulnakmol/agentic-ayudha/issues/885),
  [#886](https://github.com/rahulnakmol/agentic-ayudha/issues/886) and
  [#889](https://github.com/rahulnakmol/agentic-ayudha/issues/889) ([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Links).
  All four are open and `status:planned`. #885 is in milestone v0.4.1, not v0.4.0 (GitHub, read 2026-11-07).
- Its Stories are made Ready only after the intent, spec, and UX design baselines are approved, the prototype states
  pass Hanu's review (D4), and the Feature passes Approve to build ([#877](https://github.com/rahulnakmol/agentic-ayudha/issues/877))
  ([#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976) Scope). None of these steps is recorded.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Specification](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md): the planned web and phone screens, the most important file for this Feature
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
