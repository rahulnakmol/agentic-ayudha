# Implementation plan: Feature #875

## All Stories and Bugs

Labels from `_data/subs.tsv`. No row of `_data/merge-refs.tsv` or `_data/open-prs.tsv` refs these issues.

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| [#919](https://github.com/rahulnakmol/agentic-ayudha/issues/919) | The reviewer table and one finding type are in the registry, and every review timing is a config key | story | planned | S | high | #874 | `internal/verbs/`, `internal/workitems/critique`, `internal/config/`, `docs/RACI.md` (generated) | not started |
| [#920](https://github.com/rahulnakmol/agentic-ayudha/issues/920) | aayu critique records one review per seat and refuses a review that is not independent | story | planned | M | high | #874, #919 | `internal/verbs/registry.go`, `internal/cli/critique.go`, `skills/critique/SKILL.md` | not started |
| [#921](https://github.com/rahulnakmol/agentic-ayudha/issues/921) | An approved baseline starts one review session for each reviewing seat on the approver's agent tool | story | planned | M | medium | #874, #920 | `internal/cli/ready.go`, `internal/delegate/` | not started |
| [#922](https://github.com/rahulnakmol/agentic-ayudha/issues/922) | A slow tick starts each missed review, and the review window closes on time | story | planned | S | medium | #874, #921 | `internal/cli/critique.go`, `loops/baseline-reviews.toml`, rendered loop files | not started |
| [#923](https://github.com/rahulnakmol/agentic-ayudha/issues/923) | A local agent CLI runs a review session, and aayu critique records it when the session ends | story | planned | S | high | #874, #920, #921 | `internal/cli/critique.go`, `internal/delegate/local.go` | not started |

Dependency #874 is the Feature, and its five baseline Stories (#990 to #994) are merged.

## Delivered

No Story of #875 is delivered. Its baselines are: intent, spec, and tech v1 approved by `rahulnakmol` on
2026-11-02 (`specs/875/baselines/*-v1.json`), and `specs/875/design.json`. The body hash of #875 on 2026-11-07
equals the intent pin `f6e713adc3c9…` (computed with the `workitems.BodyHash` rule), so the intent has not drifted.

Pieces that other Features delivered and that #875 will use are listed in
[03-technical-design.md](03-technical-design.md) (for example `ready.ReviewPath`, `Env.CritiqueState`, and
`[critique] review_wait_hours`).

## In review

None.

## Pending

- All five Stories. None is `status:ready`.
- Approve to build of #875: a person's step. `specs/875/approved.json` does not exist on `ledger`. Until a
  person approves, `aayu ready <story>` refuses each Story with "Feature #875 is not approved to build. A person
  runs aayu ready 875." (`earlyCovered` in `internal/cli/readycover.go`).
- Work not sliced into Stories:
  - Copilot review sessions: they wait for Feature [#917](https://github.com/rahulnakmol/agentic-ayudha/issues/917)
    (v0.4.1), "with no story here" (#921 Assumptions).
  - Whether a Cursor, Claude Code, or Codex cloud environment can hold an aayu token is "unverified" in #921's own
    Assumptions; CG 9 covers the case where it cannot.
  - The cron may change after the first Features measure the review wait (#922 Open questions: deferred).
- No source gives an estimate beyond the effort labels (S, M, M, S, S).
