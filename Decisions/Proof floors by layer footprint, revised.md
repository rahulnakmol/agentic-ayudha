---
id: decision-floors-by-footprint-revised
type: decision
title: Proof floors by layer footprint, revised
owner: founder
people: [rahulnakmol]
status: accepted
updated: "2026-10-18"
reviewed: 2026-10-18
sources: ["Decision #485", "Decision #480", "PR #500"]
up: "[[Decisions]]"
issues: [485]
supersedes: ["[[Proof floors by layer footprint]]"]
tags: [type/decision]
---

# Proof floors by layer footprint, revised

## Context

[[Proof floors by layer footprint]] turned DAST on by default.
Its accepted text did not yet carry guardrails for DAST.
aayu has no verb that edits an issue body.
So the revision is a new Decision.

## The choice

Decision #485 keeps the choice of #480 and adds the DAST guardrails.
An active scan runs only against an app that verify starts on its own machine.
DAST applies only to web and api platforms.
A web or api Story with no runnable target records a blocking skip.
Decision #485 was accepted on 2026-10-17.

## Rejected options

- Active scans against any target: a scan changed a test app's state, so it would change shared data.
- The options rejected under #480 stay rejected.

## Consequences

Thresholds stay in protected configuration.
Only a person's protected-path change moves them.
PR #500 adds the push and nightly security scan.

## Record

<!-- BEGIN aayudha:brain:record -->
- Recorded in #485 Decision: Decision: proof floors by layer footprint, revised: \#480 with the founder's DAST guardrails \(active scan only of apps verify starts, web and api only, Docker or Podman, measured cost\).
<!-- END aayudha:brain:record -->
