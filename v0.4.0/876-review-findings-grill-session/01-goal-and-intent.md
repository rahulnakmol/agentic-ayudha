# Goal and intent: Feature #876

## Intent baseline

`specs/876/baselines/intent-v1.json` on `ledger`: owner `product-strategist`, drafted 2026-10-31T10:05:51Z, approved
by `rahulnakmol` at 2026-11-02T17:17:40Z through `device-flow`, hash `2c8dbde9d609…`. It pins `#876 body` at hash
`a4d8ce427f69…`; the body on 2026-11-07 has the same hash (computed with the `workitems.BodyHash` rule). Its
`summary` field reads:

> `aayu grill feature <f> --findings` walks the open findings with the founder in the owner's thread, and each
> resolution is recorded. This is step S7 of the core design (aayu core: the human-dependency engine, agreed by
> @rahulnakmol on 2026-10-21).

## Goal

No Feature builds on a review gap that nobody resolved. Each gap is fixed in a later approved baseline or
dismissed by a person with a reason, before Approve to build
([#876](https://github.com/rahulnakmol/agentic-ayudha/issues/876) Problem: "Q18: every gap is fixed in a new
baseline or dismissed with a reason before Approve to build";
[#1114](https://github.com/rahulnakmol/agentic-ayudha/issues/1114) Summary).

## Who it serves and the customer outcome

- The founder. He sees every finding with its state, dismisses a gap he judges out of scope with a one-line
  reason, and sees each dismissal again at Approve to build
  ([#1114](https://github.com/rahulnakmol/agentic-ayudha/issues/1114),
  [#1115](https://github.com/rahulnakmol/agentic-ayudha/issues/1115) Summaries).
- The agent in the founder's grill session. It walks each open gap with a recommendation, drafts the fix as a new
  baseline version, and hands the person the dismissal command; it never dismisses a gap itself
  ([#1116](https://github.com/rahulnakmol/agentic-ayudha/issues/1116) Summary).

## Problem

[#876](https://github.com/rahulnakmol/agentic-ayudha/issues/876) Problem: "Nothing records a resolution today."
On `dev`, `Env.CritiqueState` is nil in production, so `Env.critique` returns no reviews and no open gaps
(`internal/cli/readyapprove.go`; `internal/cli/env.go`).

## Success measures

| Story | Measure | Threshold |
|---|---|---|
| [#1114](https://github.com/rahulnakmol/agentic-ayudha/issues/1114) | Approvals to build written while a review gap of the Feature was open, from `specs/*/approved.json` and the review records | 0 in v0.4.0 |
| [#1115](https://github.com/rahulnakmol/agentic-ayudha/issues/1115) | Dismissals on the ledger with no presence record or no reason, from `specs/*/resolutions.jsonl` | 0 in v0.4.0 |
| [#1116](https://github.com/rahulnakmol/agentic-ayudha/issues/1116) | Pass rate of the `grill/findings-walk` eval case | the grill group's bar in `evals/eval.toml` |

## Out of scope

- The inbox screens ([#876](https://github.com/rahulnakmol/agentic-ayudha/issues/876) Scope: "Out: the inbox
  screens").
- Outcomes of inputs and guidance (`taken`, `declined`): deferred, because inputs and guidance never block (Q18)
  ([#1114](https://github.com/rahulnakmol/agentic-ayudha/issues/1114),
  [#1115](https://github.com/rahulnakmol/agentic-ayudha/issues/1115) Open questions).
- Writing the review records: Feature [#875](https://github.com/rahulnakmol/agentic-ayudha/issues/875). #1114
  needs nothing from #875; its tests seed the records (#1114 Assumptions).
