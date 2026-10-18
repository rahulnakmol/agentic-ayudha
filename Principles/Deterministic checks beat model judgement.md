---
id: principle-deterministic-checks
type: principle
title: Deterministic checks beat model judgement
owner: founder
people: [rahulnakmol]
status: current
updated: 2026-10-18
reviewed: 2026-10-18
sources: ["file ETHOS.md", "file CONSTRAINTS.md", "file docs/BUILD-PLAN.md"]
up: "[[Principles]]"
tags: [type/principle]
---

# Deterministic checks beat model judgement

## The principle

A floor miss blocks the merge.
No model disposition can override it.
Only a recorded Decision can waive a miss, and audit counts every waiver.

## Why

A model can be wrong and still sound sure.
A fixed check with a fixed threshold gives the same answer every time.
Ground rule 7 of the build plan states the same rule.

## In practice

The floors cover coverage, SAST, secrets, dependencies, accessibility, and performance.
Their thresholds are in CONSTRAINTS.md.
See [[Floor (term)]] and [[Waiver (term)]].
