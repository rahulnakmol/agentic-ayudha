# Goal and intent: Feature #975

## Source note

There is no intent baseline. `git ls-tree -r --name-only origin/ledger specs/975` lists no files, so there is no
`intent-v1.json` and no `.draft` to quote. Everything below comes from the Feature body
([#975](https://github.com/rahulnakmol/agentic-ayudha/issues/975)), its 20 sub-issues, and the parent Initiative
([#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870)). Issue #975 has no comments.

## Goal

"`aayu inbox` shows every decision, answer, and alignment a person owes the agents, in areas (Now, Overdue, Ship,
Answer, Decide, Ready, Housekeeping), and each mission's state. Any seat asks on an ANSWER card and the answer reaches
its thread; Decisions, plan reviews, blocked builds, incidents, the emergency stop, Feedback, Feature baselines,
Approve to build, triage, story reviews, overrides, and the stable release are each a card with its next step."
(issue #975, Summary)

## Who it serves and the outcome

- The founder: every Story is written "As the founder" (#937 to #956).
- The agents (seats and their twins): their questions reach a person and the answer comes back to their thread, "so
  that agents go on within a day" (#938 Summary).

Outcome: one place to steer every mission and take every decision; no agent waits more than 48 hours (#940 Summary);
"no PR wastes a day" (#949 Summary).

The Initiative's business measure applies: PRs merged into dev, baseline 527 in 28 days (2026-09-23 to 2026-10-21),
target 1,000 in 28 days by the v0.4.0 release (issue #870).

## Problem

From issue #975, Problem:

- In the v0.3.0 release, decisions ran outside the inbox: the "Stable release: vX.Y.Z" comment on a Release issue,
  Decisions and P1 declines that agents asked @rahulnakmol to confirm, and the choice between hold and review for a
  PR whose rounds kept finding bugs (#676).
- Pages that wait on a person print text but make no card (#873): a blocked build (`internal/cli/build.go`), the
  emergency stop (`internal/cli/halt.go`), and the budget pause (`internal/models/gateway.go`).
- "Decision #913 runs each Feature as a mission, and the founder needs one view of what each mission runs and what
  waits on him." Decision #913 is now closed as superseded by Decision
  [#915](https://github.com/rahulnakmol/agentic-ayudha/issues/915) (comment on #913, 2026-10-22).

## Success measures

Each Story states a measure and a threshold (each issue body, Measure and Threshold):

| Issue | Measure | Threshold |
|---|---|---|
| [#937](https://github.com/rahulnakmol/agentic-ayudha/issues/937) | Areas shown in the agreed order | 7 of 7, in order |
| [#938](https://github.com/rahulnakmol/agentic-ayudha/issues/938) | Asks answered within 24 h of being asked | at least 90% (28 days, from the ask records) |
| [#939](https://github.com/rahulnakmol/agentic-ayudha/issues/939) | Snooze, Not now, and Not sure choices that need a typed command, per release | 0 |
| [#940](https://github.com/rahulnakmol/agentic-ayudha/issues/940) | One-way asks that applied without the founder | 0 |
| [#941](https://github.com/rahulnakmol/agentic-ayudha/issues/941) | Stories made Ready that Approve to build does not cover and whose change no person acknowledged | 0 |
| [#942](https://github.com/rahulnakmol/agentic-ayudha/issues/942) | Mission starts, stops, resumes, recovers, and retries typed in a terminal, per release | 0 |
| [#943](https://github.com/rahulnakmol/agentic-ayudha/issues/943) | Stories a mission runs that its view does not show | 0 |
| [#944](https://github.com/rahulnakmol/agentic-ayudha/issues/944) | Story cards with a timeline | 100% of story cards |
| [#945](https://github.com/rahulnakmol/agentic-ayudha/issues/945) | Plan approvals and build unblocks done outside the inbox, per release | 0 |
| [#946](https://github.com/rahulnakmol/agentic-ayudha/issues/946) | Stable-release comments and publishes done by hand, per release | 0 |
| [#947](https://github.com/rahulnakmol/agentic-ayudha/issues/947) | Ops approvals that bypass GitHub's required reviewers | 0 |
| [#948](https://github.com/rahulnakmol/agentic-ayudha/issues/948) | Emergency stops and resumes typed in a terminal, per release | 0 |
| [#949](https://github.com/rahulnakmol/agentic-ayudha/issues/949) | PRs that only a floor or an eval stops, or that loop past `[inbox] loop_rounds` or `loop_hours`, with no inbox card, per release | 0 in v0.4.0 (v0.3.0: #857, #858, #866, and #896 merged by hand past eval bars; #674 and #676 looped about 41 h each and used 36% of the review spend) |
| [#950](https://github.com/rahulnakmol/agentic-ayudha/issues/950) | Feedback accepted by a hand-added label, per release | 0 |
| [#951](https://github.com/rahulnakmol/agentic-ayudha/issues/951) | Baselines approved outside the inbox, per Feature | 0 |
| [#952](https://github.com/rahulnakmol/agentic-ayudha/issues/952) | Findings decided one by one on the inbox card | 0 (they are decided in the grill session) |
| [#953](https://github.com/rahulnakmol/agentic-ayudha/issues/953) | Stories built under a Feature that was not approved to build | 0 |
| [#954](https://github.com/rahulnakmol/agentic-ayudha/issues/954) | Bugs made Ready with no triage decision | 0 |
| [#955](https://github.com/rahulnakmol/agentic-ayudha/issues/955) | Ready stories added or changed in Completion Goal, effort, or title after Approve to build, with no acknowledgment | 0 |
| [#956](https://github.com/rahulnakmol/agentic-ayudha/issues/956) | Times shown without a range and a basis | 0 |

The Summary of #940 adds the goal that "no agent waits more than 48 hours".

## Out of scope

From issue #975, Scope, "Out":

- The web faces (#976); the deck, pairing, and the Behaviours and Ask why views (#977).
- "The engines these cards show, which are the core Features this one depends on."
- For every wave: why reviews fail (#810, #813), a gate that runs an old head (#812), the superseded cancelled run
  (#818), failure wording in general (#797), SQLite, GSAP, and the open internet.

## Entry conditions stated by the Feature

"Its stories are made Ready only after this Feature's intent, spec, and UX design baselines are approved, its web and
terminal prototype states pass Hanu's review (D4), and it passes Approve to build (#877)." (issue #975, Scope). No
baseline exists on the ledger. The evidence does not show a Hanu review record for this Feature.

## UX research sources

The Stories cite "interaction specs v4" (touchpoints TP7 to TP91) and lettered answers such as C23, Q14, Q15, E3, M9,
M11, R9, J1, Y5 (for example #937, #938, #940, #942, #943, #949 Assumptions). Those documents are not in this
repository or on the ledger branch (searched). Their content here is only what the issue bodies quote. A search of
GitHub issues and PRs for "Superhuman" and "HEY inbox" found no match, so this record makes no claim about those
apps.
