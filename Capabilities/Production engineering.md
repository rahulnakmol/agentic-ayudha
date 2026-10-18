---
id: capability-production-engineering
type: capability
title: Production engineering
owner: product-strategist
people: [rahulnakmol]
status: live
updated: 2026-10-18
reviewed: 2026-10-18
sources: ["Decision #324", "PR #343", "PR #352", "PR #358", "PR #560", "PR #590", "file seats/production-engineer/persona.toml"]
up: "[[Capabilities]]"
objectives: ["[[Ship aayudha 0.1]]"]
tags: [type/capability]
---

# Production engineering

## What it does

investigate follows an incident from bounded telemetry to the deployed code.
slo defines SLOs and gates releases on the error budget.
runbook checks each runbook's alerts against the configured projects.
respond runs an incident and writes a blameless postmortem.
Reads can come from Sentry and PostHog.

## Why it exists

Building software and running it are different work.
See [[A seat for production engineering]].

## Who it serves

It serves the Production Engineer seat and the person on call.
A change to production stays behind the catalog and a person's approval.
