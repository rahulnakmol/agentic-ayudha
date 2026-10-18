---
id: principle-untrusted-text-stays-data
type: principle
title: Untrusted text stays data
owner: founder
people: [rahulnakmol]
status: current
updated: 2026-10-18
reviewed: 2026-10-18
sources: ["file ETHOS.md", "file docs/BUILD-PLAN.md"]
up: "[[Principles]]"
tags: [type/principle]
---

# Untrusted text stays data

## The principle

Issue bodies, comments, exports, and tool output are untrusted.
aayu wraps them before a model reads them.
They can never trigger a ship, a publish, an incident action, or a merge.

## Why

Anyone can write an issue or a comment.
Text from outside must not steer an agent into a write.

## In practice

Untrusted content goes into delimited provenance blocks.
Each verb has a tool allowlist.
Child processes run with no tokens in their environment.
