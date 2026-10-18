---
id: capability-proof-packs-floors
type: capability
title: Proof packs and floors
owner: product-strategist
people: [rahulnakmol]
status: live
updated: "2026-10-18"
reviewed: 2026-10-18
sources: ["Feature #32", "file docs/guide/concepts-proof.md", "file docs/guide/concepts-floors.md", "file CONSTRAINTS.md"]
up: "[[Capabilities]]"
issues: [32]
objectives: ["[[Ship aayudha 0.1]]", "[[Prove every Story of aayu itself]]"]
tags: [type/capability]
---

# Proof packs and floors

## What it does

The merge gate in CI writes a proof pack for one exact commit.
The pack maps each Completion Goal criterion to its evidence.
It also records the result of each floor.
A floor is a fixed check with a fixed threshold, and a miss blocks the merge.

## Why it exists

An agent's "done" is a claim.
A pack that CI writes is evidence.
See [[Proof, not claims]].

## Who it serves

It serves the person who must decide whether a change is safe to merge.
It is Level 2 of adoption, with no AI key in CI.
See [[Proof pack (term)]] and [[Floor (term)]].

## Implementation history

<!-- BEGIN aayudha:brain:history -->
- Delivered in [[v0.1.0]] by #41, #42, #43, #44, #45, #46, #47, #48, #49, #50.
<!-- END aayudha:brain:history -->

## Record

<!-- BEGIN aayudha:brain:record -->
- Recorded in #32 Feature: M4 Proof.
<!-- END aayudha:brain:record -->
