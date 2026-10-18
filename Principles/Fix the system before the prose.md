---
id: principle-fix-the-system
type: principle
title: Fix the system before the prose
owner: founder
people: [rahulnakmol]
status: current
updated: 2026-10-18
reviewed: 2026-10-18
sources: ["file ETHOS.md"]
up: "[[Principles]]"
tags: [type/principle]
---

# Fix the system before the prose

## The principle

When an agent gets something wrong, fix the highest layer that can make the mistake impossible.
First fix the codebase with types, invariants, and boundaries.
Then add static checks, such as lint rules, tests, CI checks, and verify.
Use prose only for what needs judgement.
Put the least effort into human review.

## Why

A rule in code catches a mistake for good.
A rule in prose relies on someone reading it.

## In practice

retro records the layer of each correction, with its reason.
A codebase or check correction becomes a Story with a new test or rule as proof.
