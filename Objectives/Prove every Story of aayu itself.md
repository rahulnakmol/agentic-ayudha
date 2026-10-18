---
id: objective-dogfood-proof-coverage
type: objective
title: Prove every Story of aayu itself
owner: product-strategist
people: [rahulnakmol]
status: active
updated: 2026-10-18
reviewed: 2026-10-18
sources: ["issue #30", "file docs/BUILD-PLAN.md"]
up: "[[Objectives]]"
direction: "[[Build aayu with aayu]]"
issues: [30]
tags: [type/objective]
---

# Prove every Story of aayu itself

## The objective

aayu builds the rest of aayu with aayu.
Each remaining work package is a Story with a Completion Goal.
Each one is made Ready, and from M4.5 on it ships with an authoritative proof pack.
Initiative #30 is the record of this work.

## How we measure it

The metric is proof coverage.
It divides done Stories with a holding authoritative pack by all done Stories.
The target is 100 % at work package 8.1.

## Why it matters

If aayu cannot prove its own work, it cannot ask users to prove theirs.
Every merge here must pass the gates that aayu builds.
See [[Proof packs and floors]].
