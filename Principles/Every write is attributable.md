---
id: principle-every-write-attributable
type: principle
title: Every write is attributable
owner: founder
people: [rahulnakmol]
status: current
updated: 2026-10-18
reviewed: 2026-10-18
sources: ["file ETHOS.md", "file docs/BUILD-PLAN.md"]
up: "[[Principles]]"
tags: [type/principle]
---

# Every write is attributable

## The principle

External writes go through one actions log with an idempotency key.
GitHub's own event history is the audit source of record.
Bypasses are explicit and listed, never silent.

## Why

An audit must show who wrote what, and when.
A silent write or bypass breaks that record.

## In practice

Audit reconciles the actions log against GitHub's event history.
It flags writes that are not in the log.
See [[The ledger]].
