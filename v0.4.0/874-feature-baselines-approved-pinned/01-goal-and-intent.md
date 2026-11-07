# Goal and intent: Feature #874

## Intent baseline

The intent baseline is `specs/874/baselines/intent-v1.json` on `ledger`. Owner seat `product-strategist`, drafted
2026-10-30T18:27:57Z, approved by `rahulnakmol` at 2026-10-30T19:09:28Z through `device-flow`, hash
`914c5edd93d5…`. It pins one thing: `#874 body` at body hash `5f052d2259d6…`. Its summary reads:

> Intent, spec, technical design, and UX design each get a baseline: an agent drafts it, a person approves it, and
> later edits show as drift. This is step S5 of the core design (aayu core: the human-dependency engine, agreed by
> @rahulnakmol on 2026-10-21).

The Feature's Ready snapshot (`specs/874/ready.md`) pins the same body hash, so the body that the intent baseline
froze is the body at Ready.

## Goal

Freeze what a person agreed for each part of a Feature, so that an agent cannot approve it for the person and a
later change shows ([#874](https://github.com/rahulnakmol/agentic-ayudha/issues/874) Summary;
[#990](https://github.com/rahulnakmol/agentic-ayudha/issues/990) Summary: "so that what I agreed is frozen and
cannot be approved for me").

## Who it serves and the customer outcome

- The founder, @rahulnakmol. He approves each baseline once, at the hash he saw, and he approves a new version
  knowing what changed, why it matters, and the need behind it
  ([#990](https://github.com/rahulnakmol/agentic-ayudha/issues/990),
  [#993](https://github.com/rahulnakmol/agentic-ayudha/issues/993) Summaries).
- Vishwa, the Code Architect twin. `aayu plan` takes a Feature and writes one technical design that every story
  plan stays inside ([#991](https://github.com/rahulnakmol/agentic-ayudha/issues/991) Summary).
- Hanu, the UX twin. One flagged early-build story can run before Approve to build, so a design can be shown
  running while nothing else spends build tokens
  ([#994](https://github.com/rahulnakmol/agentic-ayudha/issues/994) Summary).
- The founder at Approve to build. He decides only on the changes that a revert cannot undo, from each story's risk
  register ([#1028](https://github.com/rahulnakmol/agentic-ayudha/issues/1028) Summary).

## Problem

From the Feature body ([#874](https://github.com/rahulnakmol/agentic-ayudha/issues/874) Problem): "Only the story
Ready snapshot and the release scope baseline freeze anything (`internal/ready/ready.go:146-303`). `aayu plan`
refuses a Feature (`internal/cli/plan.go:144-146`) (G5, Q16)." Those line numbers are from before the work; on
`dev` now `aayu plan` takes a Feature (`planFeature` in `internal/cli/plan.go`).

## Success measures

Each Story states its own measure and threshold:

| Story | Measure | Threshold |
|---|---|---|
| [#990](https://github.com/rahulnakmol/agentic-ayudha/issues/990) | Baseline approvals written without a person's device-flow check, and approvals at a hash other than the one shown | 0 of each over every seeded run of the six criteria |
| [#991](https://github.com/rahulnakmol/agentic-ayudha/issues/991) | Story plans that pass with a checkpoint layer outside their Feature's technical design | 0 over the seeded runs of criteria 6 and 7 |
| [#992](https://github.com/rahulnakmol/agentic-ayudha/issues/992) | Pins whose hash changes when only text outside the pinned fields changes | 0 over the seeded runs of criteria 1 and 2 |
| [#993](https://github.com/rahulnakmol/agentic-ayudha/issues/993) | New baseline versions written with a change that has no what, why, or need | 0 over the seeded runs of criteria 2 to 4 |
| [#994](https://github.com/rahulnakmol/agentic-ayudha/issues/994) | Early-build approvals that cover more than one story, or do not show their budget before the device code | 0 over the seeded runs of criteria 1 and 2 |
| [#1028](https://github.com/rahulnakmol/agentic-ayudha/issues/1028) | Completion Goal criteria observed | all 8 criteria |
| [#1106](https://github.com/rahulnakmol/agentic-ayudha/issues/1106) | Completion Goal criteria observed | all 8 criteria |

The Feature body itself states no Feature-level measure.

## Out of scope

- The seat reviews of each baseline: core design step S6, Feature
  [#875](https://github.com/rahulnakmol/agentic-ayudha/issues/875)
  ([#874](https://github.com/rahulnakmol/agentic-ayudha/issues/874) Scope: "Out: the seat reviews (S6)").
- Closing a review finding. [#993](https://github.com/rahulnakmol/agentic-ayudha/issues/993) records `--fixes`
  inside the hash, but "#874 never closes a finding, #876 does" (its Review guidance).
- The Ready rule that accepts an early-build record for its one story is #877's
  ([#994](https://github.com/rahulnakmol/agentic-ayudha/issues/994) Assumptions); it is delivered in
  [#996](https://github.com/rahulnakmol/agentic-ayudha/issues/996).
