# Goal and intent: Review guidance

## Goal

The intent baseline v1 (`specs/887/baselines/intent-v1.json` on the ledger, hash `503b0ea49b2c`, approved by
@rahulnakmol on 2026-11-02T17:20:23Z through the device flow) records this summary:

> A `review_guidance` field (check, risk, ignore) is frozen with the story, and the build brief and inspect read it.
> This is step S18 of the core design (aayu core: the human-dependency engine, agreed by @rahulnakmol on 2026-10-21).

The baseline pins the #887 body at hash `d85b025a…` (`pins[0]`). The body is the full statement of intent.

## Who it serves and the outcome

- The person who approves work (@rahulnakmol in each Story's "As @rahulnakmol" line,
  [#1221](https://github.com/rahulnakmol/agentic-ayudha/issues/1221),
  [#1222](https://github.com/rahulnakmol/agentic-ayudha/issues/1222),
  [#1223](https://github.com/rahulnakmol/agentic-ayudha/issues/1223)).
- The builder agent: it "proves what the reviewer will check before the first push" (#1222 Summary).
- The reviewer (`aayu inspect`): it "looks first at the named risk and never drops a finding that matters" (#1223
  Summary).

## Problem

"inspect reads no per-story guidance and no `plan.json` (`internal/cli/inspect.go:323-350`) (G16, C33)" (#887
Problem). On `dev` at 884d81b, inspect still reads the live issue body as `StoryBody` and only the criteria from
the Ready snapshot (`internal/cli/inspect.go:412` and `:426-427`). Decision
[#864](https://github.com/rahulnakmol/agentic-ayudha/issues/864) gives the evidence: "An analysis of 544 PRs found
two things. Rounds often run after inspect has already said merge. Reviews also reopen choices that were already
settled." (#864 Question).

## Success measures

- Each Story's Measure is "Completion Goal criteria observed", with threshold "all 6 criteria" (#1221), "all 4
  criteria" (#1222), and "all 6 criteria" (#1223).
- Decision #864 names the signal that would change the rule: "if the dropped findings count shows that Ignore hid a
  finding that a person later fixed, narrow the categories" (#864 Recommendation).
- The Feature names no numeric target of its own. The Initiative target is 1,000 PRs merged into dev in 28 days
  (#870 Target); the evidence does not show how much #887 adds to it.

## Out of scope

- "the advisory reviewers' own settings" (#887 Scope, Out).
- The issue forms do not change: the guidance stays as a `#### Review guidance` block under Assumptions
  (`specs/887/design.json`, contract 1; #1221 Notes).
- A missing block only warns for now (#1221 Provenance, item 4).
