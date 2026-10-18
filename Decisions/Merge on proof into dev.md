---
id: decision-merge-on-proof
type: decision
title: Merge on proof into dev
owner: founder
people: [rahulnakmol]
status: accepted
updated: "2026-10-18"
reviewed: 2026-10-18
sources: ["Decision #593", "Decision #572", "PR #617", "PR #621", "file .github/CODEOWNERS"]
up: "[[Decisions]]"
issues: [593]
tags: [type/decision]
---

# Merge on proof into dev

## Context

Agent pull requests into dev waited for a person, one at a time.
rahulnakmol asked for merges into dev without that friction.
Promotion to rolling and main needs no human approval.
So a change merged into dev can reach main with no human step.

## The choice

aayu ship merges an agent pull request into dev when the proof holds.
Every required check is green, the pack verifies, and inspect says merge.
A narrow set of paths still needs a person for each pull request.
That set holds who may approve, the gate itself, releases, and secrets.
Decision #593 records it, accepted on 2026-10-18.

## Rejected options

- Merge on proof with no exception: an agent could change the gate with no person seeing it.
- A minimal set without the gate's own code: a change could weaken ship's checks unseen.

## Consequences

The streak, never-layers, and plan review stop blocking a merge into dev.
They become a sampled audit after merge.
The narrow set lives in protected configuration, so only a person can move it.
It replaces the per-pull-request passes of [[A person's attention by exception]] for dev.
PR #617 and PR #621 build it.

## Record

<!-- BEGIN aayudha:brain:record -->
- Recorded in #593 Decision: Agent PRs into dev merge on proof through aayu ship; which narrow set still needs a person (supersedes #572 for dev).
<!-- END aayudha:brain:record -->
