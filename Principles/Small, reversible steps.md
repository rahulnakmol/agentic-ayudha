---
id: principle-small-reversible-steps
type: principle
title: Small, reversible steps
owner: founder
people: [rahulnakmol]
status: current
updated: 2026-10-18
reviewed: 2026-10-18
sources: ["file ETHOS.md", "file docs/BUILD-PLAN.md"]
up: "[[Principles]]"
tags: [type/principle]
---

# Small, reversible steps

## The principle

Work moves in Stories of size XS to L, checkpoint by checkpoint.
It goes through dev, then rolling, then main.
Every release is signed.
You can trace each release to the commit and workflow that built it.

## Why

A small step is easy to check and easy to undo.

## In practice

The promotion ladder runs a gate at each step.
See [[Release and promotion]].
