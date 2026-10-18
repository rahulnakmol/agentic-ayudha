---
id: capability-inspect-review
type: capability
title: Inspect and review
owner: product-strategist
people: [rahulnakmol]
status: live
updated: "2026-10-18"
reviewed: 2026-10-18
sources: ["Feature #32", "Feature #34", "Decision #469", "Decision #544", "file docs/guide/how-merge-gate.md", "file seats/ux-researcher-evolutionist/persona.toml"]
up: "[[Capabilities]]"
issues: [32, 34]
objectives: ["[[Ship aayudha 0.1]]"]
tags: [type/capability]
---

# Inspect and review

## What it does

inspect reviews every pull request that is not docs-only.
A reviewer of another model family reviews it where one is available.
inspect gives a disposition: merge, fix, or rebuild.
review critiques a change with a UI against its goals.

## Why it exists

The agent that wrote the code must not mark its own work.
A disposition can downgrade a result, but it never overrides a floor.

## Who it serves

It serves the person who owns the merge, and the builder agent that must fix findings.
See [[The review ladder]] and [[Inspect's review converges]].

## Implementation history

<!-- BEGIN aayudha:brain:history -->
- Delivered in [[v0.1.0]] by #41, #42, #43, #44, #45, #46, #47, #48, #49, #50, #59, #60, #61.
<!-- END aayudha:brain:history -->

## Record

<!-- BEGIN aayudha:brain:record -->
- Recorded in #32 Feature: M4 Proof.
- Recorded in #34 Feature: M6 UX.
<!-- END aayudha:brain:record -->
