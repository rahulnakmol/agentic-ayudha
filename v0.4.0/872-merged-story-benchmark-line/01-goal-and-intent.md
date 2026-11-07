# Goal and intent: one benchmark line per merged story

No intent baseline exists (`specs/872/` is absent on `origin/ledger`). The source is the #872 body.

## Goal

"`ship` appends one line per story to `delivery/stories.jsonl` at merge, and `release` appends one line per release
to `delivery/releases.jsonl`." (#872 Summary). Step S3 of the core design (#872 Summary).

## Who it serves and the outcome

- aayu itself, to estimate: "aayu cannot estimate a story, size `in_flight`, or propose a budget (G17)" (#872
  Problem). `in_flight` was later replaced by `[build] agents` (Decision
  [#918](https://github.com/rahulnakmol/agentic-ayudha/issues/918), accepted 2026-10-23; PR
  [#1089](https://github.com/rahulnakmol/agentic-ayudha/issues/1089)).
- Feature [#886](https://github.com/rahulnakmol/agentic-ayudha/issues/886), which learns time and cost from
  `delivery/stories.jsonl` (#886 Scope).
- The person who approves a Feature's budget: today the mission counts every story at its per-story cap, "until #872
  measures spend" (`internal/cli/mission.go:1399-1406`).

## Problem

"No record of story times exists … The data must start early so that v0.4.0 has samples." (#872 Problem). On `dev`
at 884d81b, the closest records are each PR's ship record (`proof/<story>/<head>/authoritative/ship.json`, with
`shipped_at`; `internal/cli/ship.go:67` `ShipRecord`) and each release rung (`releases/<n>/<target>.json`,
`internal/cli/release.go:633` `rungPath`). Neither holds the story's earlier times, rounds, or cost.

## Success measures

The issue names no measure or threshold. A sample count matters to #886: Decision #868 (cancelled) planned a 3 h
default "with fewer than 5 samples" (#868 Options). The evidence does not show a sample rule that still holds.

## Out of scope

"the delivery plan view (S17)" (#872 Scope, Out).
