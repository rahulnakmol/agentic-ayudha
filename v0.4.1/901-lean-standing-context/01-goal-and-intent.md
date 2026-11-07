# Goal and intent: #901 lean standing context

No intent baseline exists (no `specs/901/` on `origin/ledger`, 2026-11-07). The source is
[#901](https://github.com/rahulnakmol/agentic-ayudha/issues/901), checked against `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b).

## Goal

"aayu cuts the standing context on the build and inspect path by experiment: a cut that holds every eval bar stays, and
verbs and loops unused for 30 days are removed." (#901, Summary.)

## Who it serves

Every repository that runs aayu builds and reviews. The issue says "More context costs tokens and time on every story"
(#901, Problem). The person who pays for model use benefits first. Inferred from the problem text.

## Problem

The issue states (#901, Problem):

- "Vercel measured about 60 kB of standing context per build session, and an 8 kB index beat full skills, 100%
  against 53 to 79% (Vercel fit-gap R7, F6)." The fit-gap note is a private research note. Unverified here.
- "aayu loads full skills and charters into each session, and no measure shows which parts change an outcome."

What `dev` shows:

| Item | Size or fact on 884d81b | Source |
|---|---|---|
| `skills/build/SKILL.md` | 20,149 bytes | `wc -c` |
| `skills/inspect/SKILL.md` | 9,219 bytes | `wc -c` |
| All of `skills/` | 754,415 bytes of files | sum of blob sizes, `git ls-tree -r -l origin/dev skills/` |
| All of `seats/` (charters and personas) | 244,402 bytes of files | sum of blob sizes, `git ls-tree -r -l origin/dev seats/` |
| `AGENTS.md` of this repository | 25,551 bytes | `wc -c` |
| inspect's context section | capped at 40,000 bytes | `internal/inspect/context.go:31`, `maxContextBytes` |
| A measure of standing context per session | none found | search of `internal/` for a context-size measure |

These are file sizes, not what a host loads into one session. How much of each file a host loads is the host's
behaviour and was not measured. Unverified. The delegation brief does not include seat charters
(`internal/delegate/brief.go` has no charter reference).

## Success measures

From #901 Scope:

1. Standing context is measured per build and inspect session.
2. An index replaces full skills only where the cut holds every eval bar ("the bars only rise", `evals/README.md`).
3. Verbs and loops unused for 30 days, by the MCP and CLI call logs, are removed.

The issue gives no target size.

## Out of scope

"Lowering any eval bar" (#901, Scope Out). The repository already enforces this: the coverage ratchet step fails a PR
that lowers a skill-eval bar, and only an accepted `eval-bar` waiver lowers one (`AGENTS.md`, Coverage ratchet).
