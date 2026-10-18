---
id: direction-review-in-loop
type: direction
title: Review inside the delivery loop
owner: founder
people: [rahulnakmol]
status: current
updated: 2026-10-18
reviewed: 2026-10-18
sources: ["Decision #656", "Decision #654", "issue #655", "issue #659", "issue #648", "PR #641"]
up: "[[Direction]]"
objections: ["[[Keep review apps as an option]]"]
tags: [type/direction]
---

# Review inside the delivery loop

## Where we are going

By default, the build agent reviews its change in its own session before it opens a pull request.
A reviewer model from a shipped pairing table does that review.
Then the merge gate re-runs the same review and floors once in CI.
The pull request merges when that pass holds.
Review apps stay supported as a mode that a repository chooses.
Issues #655, #659, and #648 track the work.

## Why now

Review apps can run out of quota.
They add rounds of findings and need setup for each account.
Decision #656 says to plan the 0.4 work against this choice.

## What we will not do

We will not merge on a review written only inside the build session.
That review cannot be told apart from the agent itself.
This change is not in v0.3.0, which ships the current design of PR #641.
See [[v0.3.0]].
