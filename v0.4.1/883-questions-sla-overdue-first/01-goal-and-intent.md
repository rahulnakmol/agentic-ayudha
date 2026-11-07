# #883 Goal and intent

No intent baseline exists for this Feature: `git ls-tree origin/ledger specs/` lists no `specs/883`. This page uses
[#883](https://github.com/rahulnakmol/agentic-ayudha/issues/883), Decision
[#862](https://github.com/rahulnakmol/agentic-ayudha/issues/862), and Story
[#940](https://github.com/rahulnakmol/agentic-ayudha/issues/940) (under Feature #975, v0.4.0).

## Goal

Quoted from #883: "`decideSweep` walks questions as well as Decisions: Overdue at 24 h, then a provisional answer
at 48 h for two-way questions only."

## Who it serves and the customer outcome

- The founder: #940's Summary says "I want to see what is about to block the team and what is overdue, with a
  banner that tells me to take those first, so that no agent waits more than 48 hours."
- The agent seats that ask: they get an answer, or a provisional one for a two-way question, within 48 business
  hours (#862 option 1).

## Problem

Quoted from #883: "Only Decisions have an SLA (`internal/cli/decide.go:171-273`) (G13, Q15). Decision #862."

Decision #862 adds: "Questions do not exist yet (inbox design Q15; core design D3, S14)." The code confirms it: no
`ask` verb in `internal/verbs/registry.go`, no `internal/cli/ask.go`.

## Scope

In, quoted from #883: "the Overdue area and banner; one page at 24 h; provisional apply for two-way questions, shown
in Decide to ratify or revert; a snooze past due needs a reason."

## Out of scope

Quoted from #883: "changing the Decision SLA itself."

## Success measures

The sources state no success measure. Decision #862 names a review trigger: "if more than a few provisional answers
are reverted each month, lengthen the window, or make each seat's questions one-way by default."

## Dependencies

Issue #883 links `depends-on` [#873](https://github.com/rahulnakmol/agentic-ayudha/issues/873) (One Need registry) and
[#880](https://github.com/rahulnakmol/agentic-ayudha/issues/880) ("Any seat asks through aayu ask, and the first
answer wins"). Neither is sliced. Story [#1106](https://github.com/rahulnakmol/agentic-ayudha/issues/1106) (v0.4.1,
under #874) depends on this Feature's SLA for its open questions (Release #1105 Summary).
