---
id: direction-merge-on-proof
type: direction
title: Merge on proof, people by exception
owner: founder
people: [rahulnakmol]
status: current
updated: 2026-10-18
reviewed: 2026-10-18
sources: ["Decision #572", "Decision #593", "PR #577", "PR #617", "PR #621", "PR #587"]
up: "[[Direction]]"
tags: [type/direction]
---

# Merge on proof, people by exception

## Where we are going

An agent's pull request into dev merges when the proof holds.
Every required check is green, the pack verifies, and inspect says merge.
A person looks at intent before work starts, and at a sample after merge.
A narrow set of paths still waits for a person on each pull request.
One inbox batches what still needs a person.

## Why now

On 2026-10-17 and 2026-10-18, 100 pull requests merged into dev in about 12 hours.
Under the old rules a person had to pass 90 of them, one at a time.
Decision #572 and Decision #593 record the change.
PR #617 builds merge on proof, and PR #621 adds its keys, off by default.

## What we will not do

We will not let a model override a floor.
We will not merge changes to who may approve, the gate, releases, or secrets without a person.
Promotion to rolling and main keeps its own gates.
See [[Merge on proof into dev]] and [[A person's attention by exception]].
