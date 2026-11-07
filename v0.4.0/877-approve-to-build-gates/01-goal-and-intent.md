# Goal and intent: Feature #877

## Intent baseline

`specs/877/baselines/intent-v1.json` on `ledger`: owner `product-strategist`, drafted 2026-10-30T18:28:19Z, approved
by `rahulnakmol` at 2026-11-02T17:19:49Z through `device-flow`, hash `c0156e96c4b9…`. It pins `#877 body` at hash
`da3887875545…`; the body on 2026-11-07 has the same hash (computed with the `workitems.BodyHash` rule). Its
`summary` field reads:

> `aayu ready <feature>` (human-only) is Approve to build. One approval per Feature agrees its baselines, its
> budget, and every story listed then (Decision #903). A story is Ready only while that approval covers it. A story
> added or changed later needs a person's acknowledgment, `aayu ready <feature> --ack <stories>`. This is steps S8
> and S9 of the core design (aayu core: the human-dependency engine, agreed by @rahulnakmol on 2026-10-21). This
> Feature replaces #878, which #903 folds into it.

## Goal

No story under a Feature is built until a person agrees all of it once, and Arjun makes Ready only what the person
agreed ([#995](https://github.com/rahulnakmol/agentic-ayudha/issues/995),
[#996](https://github.com/rahulnakmol/agentic-ayudha/issues/996) Summaries).

## Who it serves and the customer outcome

- The founder. One approval per Feature shows its stories, approved designs, and budget, and he sees only the
  high-risk changes by story, choosing for each one, so that after Start the mission merges without him except
  where he asked to look ([#995](https://github.com/rahulnakmol/agentic-ayudha/issues/995),
  [#1029](https://github.com/rahulnakmol/agentic-ayudha/issues/1029) Summaries).
- The founder, against forgery. Each approval record that Ready trusts carries a receipt that GitHub shows he
  wrote, so no agent with the bot identity or his `gh` login in an orb can write his agreement
  ([#1083](https://github.com/rahulnakmol/agentic-ayudha/issues/1083) Summary).
- Arjun, the Product Strategist twin. Ready refuses a story that the approval does not cover and names the next
  command ([#996](https://github.com/rahulnakmol/agentic-ayudha/issues/996)).
- Anyone who files a Story with `aayu grill story`. Grill must not file a Story that can never be Ready
  ([#1086](https://github.com/rahulnakmol/agentic-ayudha/issues/1086)).

## Problem

From [#877](https://github.com/rahulnakmol/agentic-ayudha/issues/877) Problem (line numbers as of the issue, before
this work): "Ready checks only the parent's type, never its state (`internal/workitems/validate.go:105-115`) (G7,
Q17). `ready` is a bot verb (`internal/verbs/registry.go:831-843`), and on 2026-10-20 an agent made the P1
Bug #846 Ready under the founder's name (G8). Build and the Ready guard trust any comment with the snapshot marker
(`internal/ready/ready.go:283-303`, `internal/cli/build.go:1265`) (F2). Decisions #861 and #903."

Story #996 fixed the first part: `aayu ready <story>` now reads `specs/<f>/approved.json` from the ledger
(`storyCovered`). Whether build and the guard still trust the snapshot comment is not checked here (unverified).

## Success measures

| Story or Bug | Measure | Threshold |
|---|---|---|
| [#995](https://github.com/rahulnakmol/agentic-ayudha/issues/995) | Completion Goal criteria observed | all 7 |
| [#996](https://github.com/rahulnakmol/agentic-ayudha/issues/996) | Completion Goal criteria observed | all 7 |
| [#1029](https://github.com/rahulnakmol/agentic-ayudha/issues/1029) | Completion Goal criteria observed | all 8 |
| [#1082](https://github.com/rahulnakmol/agentic-ayudha/issues/1082) | Completion Goal criteria observed | all 5 |
| [#1083](https://github.com/rahulnakmol/agentic-ayudha/issues/1083) | Completion Goal criteria observed | all 7 |
| [#1086](https://github.com/rahulnakmol/agentic-ayudha/issues/1086) | Completion Goal criteria observed | all 4 |

## Out of scope

From [#877](https://github.com/rahulnakmol/agentic-ayudha/issues/877) Scope, "Out": the inbox cards
([#895](https://github.com/rahulnakmol/agentic-ayudha/issues/895)); the early-build record (#874, delivered
in Story #994); the Bug triage rule ([#879](https://github.com/rahulnakmol/agentic-ayudha/issues/879)); the parent record
([#889](https://github.com/rahulnakmol/agentic-ayudha/issues/889)); a stop on the Feature's measured spend
([#886](https://github.com/rahulnakmol/agentic-ayudha/issues/886)). Each Story also ignores "other stories' lines
on the same approval: the Outcome line (#985), time, cost and the lock (#889, #953), and Start / Not now
(#953, #942)" (#995 and #1029 Review guidance).
