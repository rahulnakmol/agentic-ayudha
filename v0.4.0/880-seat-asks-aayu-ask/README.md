# Any seat asks through aayu ask, and the first answer wins

Feature [#880](https://github.com/rahulnakmol/agentic-ayudha/issues/880) · milestone v0.4.0 · Release
[#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906) · label `status:planned` · priority P1.

## Summary

A new `aayu ask` verb is to post one structured question on an item. A person answers it in the inbox, in the agent
tool (two-way only), or by an `/aayu answer` reply. The earliest verified answer wins (#880 Summary and Scope). This
is step S11 of the core design under Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870).
It depends on [#871](https://github.com/rahulnakmol/agentic-ayudha/issues/871) and
[#873](https://github.com/rahulnakmol/agentic-ayudha/issues/873). Features
[#881](https://github.com/rahulnakmol/agentic-ayudha/issues/881),
[#882](https://github.com/rahulnakmol/agentic-ayudha/issues/882), and
[#891](https://github.com/rahulnakmol/agentic-ayudha/issues/891) (v0.4.0), #883, #890, and #900 (v0.4.1), the inbox
Feature #975, and Stories #938 and #941 depend on it (`depends-on #880` in each body). The Feature is not sliced, and the verb
does not exist on `dev`: `aayu ask` prints `unknown command "ask" for "aayu"` (binary built from 884d81b). Two hooks
for it are on `dev` and wait for it: `Env.answeredAsk` tells a mission's Kanha that an answered story resumes, but
"until #880 is built, nothing does" call it (`internal/cli/kanha.go:194-205`); and the delegation brief reads
`specs/<n>/answers.md`, which nothing writes yet (`internal/cli/delegate.go:481`).

## Status

| Item | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | all; 5 slices proposed, not filed |
| Bugs | 0 | 0 | 0 | none named |

Sources: `_data/subs.tsv` has no row for 880; `_data/merge-refs.tsv` and `_data/open-prs.tsv` name no PR for #880;
no `specs/880/` on `origin/ledger`.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Spec](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
