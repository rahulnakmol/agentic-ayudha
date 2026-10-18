---
id: capability-ledger
type: capability
title: The ledger
owner: product-strategist
people: [rahulnakmol]
status: live
updated: "2026-10-18"
reviewed: 2026-10-18
sources: ["Feature #32", "file docs/guide/how-ledger.md", "file GLOSSARY.md"]
up: "[[Capabilities]]"
issues: [32]
objectives: ["[[Prove every Story of aayu itself]]"]
tags: [type/capability]
---

# The ledger

## What it does

The ledger is an orphan branch named ledger in the product's own repository.
It holds Ready snapshots, plans, proof packs, ship records, and audit reports.
It never holds code and is never merged.
Each record lands as one fast-forward commit.

## Why it exists

Records must live next to the work and survive any one tool.
See [[Every write is attributable]].

## Who it serves

It serves auditors and anyone who must trace a merge to its evidence.
See [[Ledger (term)]].

## Record

<!-- BEGIN aayudha:brain:record -->
- Recorded in #32 Feature: M4 Proof.
<!-- END aayudha:brain:record -->

## Implementation history

<!-- BEGIN aayudha:brain:history -->
- Delivered in [[v0.1.0]] by #41, #42, #43, #44, #45, #46, #47, #48, #49, #50.
<!-- END aayudha:brain:history -->
