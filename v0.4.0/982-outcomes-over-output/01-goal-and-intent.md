# Goal and intent: Feature #982

## The approved intent

Intent v1 was approved by @rahulnakmol on 2026-11-02T17:23:46Z through the device flow. It pins the #982 body by
hash `d211e598…` (`specs/982/baselines/intent-v1.json`, `pins`). Its summary, quoted:

> Every Feature names the customer outcome it serves, aayu asks "did it move?" after it ships, and releases and retros
> show outcomes next to counts. Each Goal states what the team does best, so new work is checked against it. As agents
> raise capacity from a few things a quarter to hundreds, the founder can see which of them made customers better off.

The same summary is in spec v1 and tech v1 (`specs/982/baselines/spec-v1.json`, `tech-v1.json`). The SHA-256 of the
current #982 body is `d211e5982f5c…`, the same as the pin, so the body has not changed since intent v1 (checked
2026-11-07).

## Who it serves and the outcome

- The founder: "the founder can see which of them made customers better off" (intent v1). Every Story is written "As
  the founder" (#984 to #989).
- Through him, the customers of each Feature: an Outcome names "who it helps"
  ([#982](https://github.com/rahulnakmol/agentic-ayudha/issues/982) Scope).
- The outcome measure the Stories use for 0.4.0 itself is founder minutes per merged PR, `aayu:founder_minutes_per_pr`
  ([#984](https://github.com/rahulnakmol/agentic-ayudha/issues/984) c2, [#989](https://github.com/rahulnakmol/agentic-ayudha/issues/989)).

## Problem

From [#982](https://github.com/rahulnakmol/agentic-ayudha/issues/982) Problem and Decision
[#981](https://github.com/rahulnakmol/agentic-ayudha/issues/981) Question:

- The founder, 2026-10-24: "will those 300 things actually make a difference … make happier customers", and "what
  really is our core competency? How can we deliver on that better?"
- aayu measures output. Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870) counts pull
  requests merged into dev (baseline 527 in 28 days, target 1,000). Its Metric is `github:prs_merged base:dev`
  (#870 body).
- #870's parent is still Goal [#29](https://github.com/rahulnakmol/agentic-ayudha/issues/29), "Ship aayudha 0.1"
  (GitHub sub-issue parent API, read 2026-11-07). Its Metric is work packages of `docs/BUILD-PLAN.md` merged (#29 body).
- The Feature form has no outcome field. The code agrees: `work-items/forms/aayudha-feature.yml` has Summary, Problem,
  Scope, Expected paths, and Links only.
- A founder-persona review of prototype v43 "found no screen that names a customer" (#982 Problem). The prototype is
  not in the repository; unverified.

## Success measures

| Story | Measure | Threshold |
|---|---|---|
| [#984](https://github.com/rahulnakmol/agentic-ayudha/issues/984) | Features in v0.4.0 created after this merges with neither an Outcome section nor `outcome:none` and a reason | 0 |
| [#985](https://github.com/rahulnakmol/agentic-ayudha/issues/985) | Approve to build texts in v0.4.0 that show no Outcome line | 0 |
| [#986](https://github.com/rahulnakmol/agentic-ayudha/issues/986) | Features merged in v0.4.0 whose outcome card never showed | 0 |
| [#987](https://github.com/rahulnakmol/agentic-ayudha/issues/987) | Release summaries in v0.4.0 that show a shipped count without outcome counts | 0 |
| [#988](https://github.com/rahulnakmol/agentic-ayudha/issues/988) | New Features in v0.4.0 with "Core: no" filed without the question being asked | 0 |
| [#989](https://github.com/rahulnakmol/agentic-ayudha/issues/989) | Windows in which `aayu measure` cannot report founder minutes per merged PR from the actions log | 0 |

## In scope

From [#982](https://github.com/rahulnakmol/agentic-ayudha/issues/982) Scope:

1. An Outcome section on the Feature form (who it helps, the Goal or Initiative metric, the expected change, and
   whether it serves the core competency), with `outcome:none` and a reason for work with no customer outcome.
2. Checks in `aayu grill feature` and Approve to build.
3. An outcome card `[outcomes] readout_days` (default 14) after a Feature's last story merges. It reads `aayu measure`
   and the linked Feedback and records moved, no change, or unknown.
4. Outcome counts on the release and in the retro.
5. A "What we do best" section on the Goal form, which triage and grill ask about.
6. A 0.4.0 Goal with a customer outcome measure that replaces Goal #29 as #870's parent. No Story covers this item
   (see [05-implementation-plan.md](05-implementation-plan.md)).

## Out of scope

From [#982](https://github.com/rahulnakmol/agentic-ayudha/issues/982) Scope:

- New analytics sources: `aayu measure` reads what it reads today.
- Experiments and readouts: `aayu experiment` and `aayu readout` stay as they are.
- Blocking a merge on an outcome: "unknown" never blocks.
- Scoring people.
- Time a person spends reading GitHub outside aayu (parked in [#989](https://github.com/rahulnakmol/agentic-ayudha/issues/989)).
