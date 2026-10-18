---
id: decision-floors-by-footprint
type: decision
title: Proof floors by layer footprint
owner: founder
people: [rahulnakmol]
status: superseded
updated: "2026-10-18"
reviewed: 2026-10-18
sources: ["Decision #480", "Decision #485"]
up: "[[Decisions]]"
issues: [480]
superseded_by: ["[[Proof floors by layer footprint, revised]]"]
tags: [type/decision]
---

# Proof floors by layer footprint

## Context

Floors checked coverage and security, but not code quality, user experience, or developer experience.
DAST was opt-in.
No workflow scanned pushes or ran nightly.

## The choice

verify picks floors by the layer footprint of a change.
Every change gets code quality floors.
A change with a UI gets user experience floors.
A change to an API or a CLI gets developer experience floors.
New floors are advisory for four weeks, then they block.
A security workflow scans every push to dev, rolling, and main, and runs nightly.

## Rejected options

- Block on every new floor at once: no time to tune the thresholds.
- Only security and DAST now: the quality, UX, and DX floors wait for a pilot.

## Consequences

Decision #480 was accepted on 2026-10-17.
Decision #485 then superseded it to add guardrails for DAST.
See [[Proof floors by layer footprint, revised]].

## Record

<!-- BEGIN aayudha:brain:record -->
- Recorded in #480 Decision: Decision: proof floors by layer footprint \(code quality, UX, API and CLI developer experience, DAST on by default\) and a push-and-nightly security workflow.
<!-- END aayudha:brain:record -->
