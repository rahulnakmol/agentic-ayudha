---
id: decision-review-ladder
type: decision
title: The review ladder
owner: founder
people: [rahulnakmol]
status: accepted
updated: "2026-10-18"
reviewed: 2026-10-18
sources: ["Decision #469", "PR #483", "PR #510", "PR #550"]
up: "[[Decisions]]"
issues: [469]
tags: [type/decision]
---

# The review ladder

## Context

The plan requires review by a model of a different family from the builder's.
Many users have only one model subscription.
For them, that rule left no valid review.

## The choice

inspect picks the highest rung that it can reach.
Rung A is a different family that a host already includes at no extra cost.
Rung B is an opt-in paid key that the user adds.
Rung C is a same-family review, labelled honestly, and it is the default.
Rung C never earns autonomy.
Decision #469 records it, accepted on 2026-10-17.

## Rejected options

- Strict different-family, fail closed: single-subscription users would turn the gate off.
- A bundled paid key: it puts an AI key and cost into every product's CI.
- Do nothing: same-family reviews would count as adversarial.

## Consequences

Each pull request records the builder's family and the rung used.
The streak counts only merges with a different-family or a person's review.
PR #483, PR #510, and PR #550 build it.

## Record

<!-- BEGIN aayudha:brain:record -->
- Recorded in #469 Decision: Decision: the review ladder: a labelled same-family review by default, a different family when a host includes one, a paid key only when added.
<!-- END aayudha:brain:record -->
