---
id: decision-attention-by-exception
type: decision
title: A person's attention by exception
owner: founder
people: [rahulnakmol]
status: accepted
updated: "2026-10-18"
reviewed: 2026-10-18
sources: ["Decision #572", "Decision #544"]
up: "[[Decisions]]"
issues: [572]
tags: [type/decision]
---

# A person's attention by exception

## Context

In about 12 hours, 100 pull requests merged into dev.
Under the rules of that time, a person had to pass 90 of them.
That load grows with every agent that opens pull requests.

## The choice

A person approves intent once for each Story, at Ready.
A narrow set of paths always needs a person for each pull request.
A sample of autonomous merges goes to the inbox after merge.
A correction resets the layer's streak and opens a revert pull request.
One inbox batches the rest.
Decision #572 records it, accepted on 2026-10-18.

## Rejected options

- Keep a pass for each pull request, batched: fewer clicks, the same number of decisions.
- Risk tiers for each pull request: the load still grows with the number of pull requests.
- Merge on proof with an audit after: intent on risky layers stays unseen.

## Consequences

A person's time goes to intent before work, and to a sample after merge.
[[Merge on proof into dev]] later removed the intent pass for dev.

## Record

<!-- BEGIN aayudha:brain:record -->
- Recorded in #572 Decision: Decision: a person's attention to agent PRs by exception: approve intent per Story, a narrow always-a-person set, sampled audit after merge, and one batched inbox.
<!-- END aayudha:brain:record -->
