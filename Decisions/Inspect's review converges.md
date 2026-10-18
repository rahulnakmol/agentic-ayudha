---
id: decision-review-converges
type: decision
title: Inspect's review converges
owner: founder
people: [rahulnakmol]
status: accepted
updated: "2026-10-18"
reviewed: 2026-10-18
sources: ["Decision #544", "PR #583"]
up: "[[Decisions]]"
issues: [544]
tags: [type/decision]
---

# Inspect's review converges

## Context

One pull request took 27 commits and 27 reviews from a reviewer app.
Each round found one more issue.
aayu's cycle cap did not stop the loop.

## The choice

The review converges within three rounds, enforced in the gate.
After round three, inspect asks the builder to step back and rethink the approach.
Later rounds review only the new diff and drop repeated findings.
A defect that the pull request did not add is tagged pre-existing and does not block it.
Decision #544 records it, accepted on 2026-10-18.
PR #583 builds the first part.

## Rejected options

- Adopt only convergence now: the reviewer gets quieter with no measure of recall.
- Rely on reviewer settings alone: nothing is enforced, and the next app loops again.

## Consequences

Later work adds responses for each finding, REVIEW.md files, an escapes eval, and risk tiers.
A merge queue waits for a trigger.
No reviewer disposition overrides a floor.

## Record

<!-- BEGIN aayudha:brain:record -->
- Recorded in #544 Decision: Decision: inspect's review converges, and the review practices it adopts from Anthropic's code review article.
<!-- END aayudha:brain:record -->
