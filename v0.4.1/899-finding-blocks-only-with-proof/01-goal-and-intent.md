# Goal and intent: #899 a finding blocks a merge only with proof

No intent baseline exists (no `specs/899/` on `origin/ledger`, 2026-11-07). The sources are
[#899](https://github.com/rahulnakmol/agentic-ayudha/issues/899) and Decision
[#910](https://github.com/rahulnakmol/agentic-ayudha/issues/910), checked against `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b).

## Goal

"A review finding blocks a merge only when it carries proof: a fix that passed in the sandbox, or a probe that fails on
the pull request. Other findings become follow-ups." (#899, Summary.) The founder agreed it on 2026-10-21 for v0.4.0
(D9, issue text; unverified here).

## Who it serves

Builders, who spend rounds on findings, and the founder, who waits for merges. The issue says "fix rounds take 68% of
the open hours" (#899, Problem).

## Problem

From #899 and the analysis in #910:

- "Builders changed code for 345 of 550 findings from reviewers that do not decide, and fix rounds take 68% of the open
  hours (review-speed analysis, #894)." (#899.) #910 gives the base: 506 of 741 open hours (68 %) were builder fixes
  and idle time between rounds, for 219 merged PRs from 2026-10-11 to 2026-10-21.
- "A finding without proof costs a round whether or not it is real, and finding precision is not measured." (#899.)
- On `dev`, a finding has a severity, a quote, a fix text, and a confidence, but no probe and no candidate patch
  (`internal/inspect/inspect.go:63-88`, `Finding`). The evidence does not show a precision measure.

## What Decision #910 already settles

Decision #910 (closed `status:done`, 2026-10-23) chose its Option 1. The rules that bear on #899:

- "A P0 or P1 blocks until its proof attempt. A prover pass writes a probe for the finding (#899). When the probe fails
  on the head, the finding blocks until it is fixed. When no probe can be written, or the probe passes, the finding is
  filed as its own Bug at its severity, and the PR can merge. A P0 on a path that `.github/CODEOWNERS` protects always
  blocks, proven or not."
- Late P2 and P3 findings become follow-ups: inspect lists them under "Follow-up", files one Bug for the PR, and keeps
  `merge`.
- A replay of PR [#828](https://github.com/rahulnakmol/agentic-ayudha/issues/828) under the proof rule: "6 of 8
  findings (3 of 5 faults) had a probe … This is an upper bound." Under a rule where an unproven P0 or P1 never blocks,
  #828 "would have merged at round 3 with 2 real P1 viewer faults".

## Success measures

From #899 Scope: a replay of #828 measures how many findings would have blocked; only a validated finding blocks;
finding precision goes into the merge record. #910 keeps the `review-escapes` eval as the check that fewer rounds did
not lower recall. The evidence does not show that eval in `evals/` or `internal/` on 884d81b.

## Out of scope

"Changing the floors; any model judgement as a blocker without proof." (#899, Scope Out.)
