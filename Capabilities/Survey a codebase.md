---
id: capability-survey
type: capability
title: Survey a codebase
owner: product-strategist
people: [rahulnakmol]
status: live
updated: 2026-10-18
reviewed: 2026-10-18
sources: ["Decision #287", "file docs/guide/skills/survey.md"]
up: "[[Capabilities]]"
objectives: ["[[Ship aayudha 0.1]]"]
tags: [type/capability]
---

# Survey a codebase

## What it does

survey analyses a repository that has code but no product docs.
A deterministic index and checks run in the binary.
The host's model drafts the analysis, the interview, and the backlog under the skill.
It can hold a modern rewrite to parity with golden outputs from the legacy build.

## Why it exists

Many codebases have no manual.
Without one, a person must read the code and grill each item by hand.

## Who it serves

It serves teams that adopt aayu on an existing or legacy codebase.
The Code Architect seat is Accountable for it.
