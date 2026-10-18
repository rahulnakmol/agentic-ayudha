---
id: capability-ship-on-proof
type: capability
title: Ship on proof
owner: product-strategist
people: [rahulnakmol]
status: live
updated: "2026-10-18"
reviewed: 2026-10-18
sources: ["Feature #32", "Feature #37", "Decision #593", "PR #617", "file AGENTS.md", "file docs/guide/journey-6-ship.md"]
up: "[[Capabilities]]"
issues: [32, 37]
objectives: ["[[Ship aayudha 0.1]]", "[[Prove every Story of aayu itself]]"]
tags: [type/capability]
---

# Ship on proof

## What it does

aayu ship merges a pull request after it checks the proof.
It checks the required checks, the pack and its floors, and inspect's disposition.
Outside a narrow set of paths, an agent pull request into dev merges on proof.
Inside the narrow set, ship marks it ready and it waits in the inbox for a person.

## Why it exists

A merge is the point where a claim becomes part of the product.
Feature #37 tracks the first run of the whole loop on one real Story.

## Who it serves

It serves teams that run many agent pull requests a day.
See [[Merge on proof into dev]].
