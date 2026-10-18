---
id: principle-one-source-of-truth
type: principle
title: One source of truth, generated everywhere else
owner: founder
people: [rahulnakmol]
status: current
updated: 2026-10-18
reviewed: 2026-10-18
sources: ["file ETHOS.md", "file LAYERS.md", "file docs/BUILD-PLAN.md"]
up: "[[Principles]]"
tags: [type/principle]
---

# One source of truth, generated everywhere else

## The principle

The verb registry and the embedded default configuration are the sources.
README tables, LAYERS.md, RACI, skill frontmatter, MCP schemas, and CONSTRAINTS.md come from them.
CI fails on drift.

## Why

Two copies of a fact drift apart.
A generated file cannot disagree with its source.

## In practice

Nobody edits a generated file by hand.
You change the source and run go generate.
LAYERS.md says at its top that the registry generates it.
See [[Verb (term)]] and [[Layer (term)]].
