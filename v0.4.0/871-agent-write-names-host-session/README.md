# Every agent write names its host and session

Feature [#871](https://github.com/rahulnakmol/agentic-ayudha/issues/871) · milestone v0.4.0 · Release
[#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906) · label `status:planned` · priority P1.

## Summary

aayu is to record the host, the session ID, and the session URL on every agent write: actions records, build state,
asks, reviews, and drafts (#871 Summary). This is step S2 of the core design under Initiative
[#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870). The source is information only, never authority
(#871 Scope, Out). The Feature is not sliced: it has no sub-issues (`_data/subs.tsv`) and no ledger records (no
`specs/871/` on `origin/ledger`). Parts of the idea exist on `dev` from other Features' work: the mission's Kanha
record names its host, session, URL, and `login@machine` (`ready.Parent`, `internal/ready/approve.go:41-55`), and a
baseline draft names its source session (`internal/ready/baseline.go:51`). The core gaps are open: an actions record
has no session field and its host is empty in a normal run, and the build state names no host or session. Features
[#872](https://github.com/rahulnakmol/agentic-ayudha/issues/872),
[#874](https://github.com/rahulnakmol/agentic-ayudha/issues/874), and
[#880](https://github.com/rahulnakmol/agentic-ayudha/issues/880) depend on this Feature (their Links). #874 lists
`depends-on #871`, yet five of its Stories merged while #871 has no Story.

## Status

| Item | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | all; 4 slices proposed, not filed |
| Bugs | 0 | 0 | 0 | none named |

Sources: `_data/subs.tsv` has no row for 871; `_data/merge-refs.tsv` and `_data/open-prs.tsv` name no PR for #871.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Spec](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
