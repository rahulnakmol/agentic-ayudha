# Goal and intent: Feature #875

## Intent baseline

`specs/875/baselines/intent-v1.json` on `ledger`: owner `product-strategist`, drafted 2026-10-31T12:05:32Z, approved
by `rahulnakmol` at 2026-11-02T17:17:15Z through `device-flow`, hash `1891e0f00bfe…`. It pins `#875 body` at hash
`f6e713adc3c9…`. Its `summary` field reads:

> A new `aayu critique` verb posts one structured review per seat, and a host-scheduled loop runs one fresh session
> per reviewing seat. This is step S6 of the core design (aayu core: the human-dependency engine, agreed by
> @rahulnakmol on 2026-10-21).

## Goal

Every approved baseline version gets an independent review from each seat that the registry names for its part,
each in a fresh session, before the Feature is approved to build
([#875](https://github.com/rahulnakmol/agentic-ayudha/issues/875) Summary and Scope;
[#920](https://github.com/rahulnakmol/agentic-ayudha/issues/920) Behaviours).

## Who it serves and the customer outcome

- The founder at Approve to build. He sees reviews that no drafting session and no owner seat wrote
  ([#920](https://github.com/rahulnakmol/agentic-ayudha/issues/920) CG 2, 3).
- The reviewing seats' twins. Each gets one brief with its focus, the pinned artifacts as untrusted data, and the
  finding schema ([#921](https://github.com/rahulnakmol/agentic-ayudha/issues/921) CG 2).
- Feature [#876](https://github.com/rahulnakmol/agentic-ayudha/issues/876). It reads the recorded findings through
  the critique state (`specs/875/design.json` contracts).

## Problem

From [#875](https://github.com/rahulnakmol/agentic-ayudha/issues/875) Problem: "No code runs a seat's review of a
baseline, nine unrelated finding types exist, and the Q16 reviewers are not the RACI's Consulted seats (G6, F5)."
The "nine finding types" claim is not checked here (unverified).

## Success measures

| Story | Measure | Threshold |
|---|---|---|
| [#919](https://github.com/rahulnakmol/agentic-ayudha/issues/919) | Baseline parts whose reviewer list differs from the Q16 table plus the mentalist on tech | 0 of 4 |
| [#920](https://github.com/rahulnakmol/agentic-ayudha/issues/920) | Reviews recorded from the drafting session or from the part's owner seat | 0 in v0.4.0 |
| [#921](https://github.com/rahulnakmol/agentic-ayudha/issues/921) | Share of review sessions started by the approval event, not the tick, over the first 10 approved baseline versions | at least 90% |
| [#922](https://github.com/rahulnakmol/agentic-ayudha/issues/922) | Missed reviews that the tick starts and that arrive inside the window | all of them, or each named as missing |
| [#923](https://github.com/rahulnakmol/agentic-ayudha/issues/923) | Local review sessions that hold a GitHub credential | 0 |

## Out of scope

- Resolving findings: step S7, Feature [#876](https://github.com/rahulnakmol/agentic-ayudha/issues/876)
  ([#875](https://github.com/rahulnakmol/agentic-ayudha/issues/875) Scope: "Out: resolving findings (S7)").
- GitHub Copilot as a review receiver. It joins when Feature
  [#917](https://github.com/rahulnakmol/agentic-ayudha/issues/917) ships its receivers, "with no story here"
  ([#921](https://github.com/rahulnakmol/agentic-ayudha/issues/921) Assumptions).
- A new config key for the agent tool. Reviews read `[repos."<owner>/<repo>"] tool` from the approver's user
  config, a key that #877's R9 story owns ([#919](https://github.com/rahulnakmol/agentic-ayudha/issues/919)
  Assumptions).
