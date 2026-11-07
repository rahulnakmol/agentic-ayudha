# The answer reaches the asking session

Feature [#881](https://github.com/rahulnakmol/agentic-ayudha/issues/881) · milestone v0.4.0 · Release
[#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906) · label `status:planned` · priority P2.

## Summary

aayu is to deliver each answer to the session that asked: a push to a cloud session, and `aayu ask --wait` for a
local session, with a receipt (sent, pull-pending, read, failed) (#881 Summary and Scope). This is step S12 of the
core design under Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870). It depends on
[#880](https://github.com/rahulnakmol/agentic-ayudha/issues/880), which is not built. The scope was narrowed on
2026-10-28: Copilot, the session-start hook, and the Claude Notification hook moved out, to v0.4.1 (#881 Scope,
Out). The Feature is not sliced. The push channel it names exists on `dev`: each of the four cloud adapters
implements `FollowUp` (`internal/delegate/receivers.go:230`, `:290`, `:393`, `:492`), and since
[7a9f3d7](https://github.com/rahulnakmol/agentic-ayudha/commit/7a9f3d7e) (#1097, 2026-10-27) aayu also uses it to
send event lines to a mission's Kanha (`Env.tellKanha`, `internal/cli/kanha.go:149-169`). No receipt and
no `--wait` exist.

## Status

| Item | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | all; 5 slices proposed, not filed |
| Bugs | 0 | 0 | 0 | none named |

Sources: `_data/subs.tsv` has no row for 881; `_data/merge-refs.tsv` and `_data/open-prs.tsv` name no PR for #881;
no `specs/881/` on `origin/ledger`.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Spec](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
