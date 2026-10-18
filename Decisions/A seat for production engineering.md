---
id: decision-production-engineer-seat
type: decision
title: A seat for production engineering
owner: founder
people: [rahulnakmol]
status: accepted
updated: "2026-10-18"
reviewed: 2026-10-18
sources: ["Decision #324", "PR #337", "file LAYERS.md", "file seats/production-engineer/persona.toml"]
up: "[[Decisions]]"
issues: [324]
tags: [type/decision]
---

# A seat for production engineering

## Context

The AI Native Engineer built the software and also wore the SRE hat.
Running software in production is different work.
It needs SLOs, investigation from telemetry, incident command, and runbooks.

## The choice

Production engineering is its own seat, with its own persona and twin.
The seat owns layer L07, and ops paths move into L07.
It is Accountable for respond and three new verbs: investigate, slo, and runbook.
The postmortem is a step of respond.
Decision #324 records it, accepted on 2026-10-17.

## Rejected options

- The same seat with no layer: L07 would stay with the AI Native Engineer.
- One new verb only: SLOs and runbooks would have no check.
- Keep the SRE hat on the AI Native Engineer: decided against.

## Consequences

The error budget becomes a deterministic gate, not an opinion.
No new path lets an agent change production.
See [[Production engineering]].

## Record

<!-- BEGIN aayudha:brain:record -->
- Recorded in #324 Decision: Decision: the Production Engineer seat (Atlas): persona, layers, verbs, telemetry access, and gates.
<!-- END aayudha:brain:record -->
