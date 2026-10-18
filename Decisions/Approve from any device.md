---
id: decision-approve-from-any-device
type: decision
title: Approve from any device
owner: founder
people: [rahulnakmol]
status: accepted
updated: "2026-10-18"
reviewed: 2026-10-18
sources: ["Decision #488", "PR #467", "PR #501"]
up: "[[Decisions]]"
issues: [488]
tags: [type/decision]
---

# Approve from any device

## Context

Human-only steps needed a device code for each command.
Agents in the cloud could not reach the person at a terminal.
An approval is only worth as much as the credential that no agent holds.

## The choice

First separate the identities, so agents act only as a bot.
Then a person approves a numbered batch of human-only steps by one comment.
The comment must come from a separate human identity.
The device code stays as the fallback and for one-way doors.
Decision #488 records it, accepted on 2026-10-17.

## Rejected options

- Device flow only, grouped: still one code for each batch, and no approval where device flow is blocked.
- Comment approvals with no identity separation: any agent with the person's token could approve.

## Consequences

The counsel in Decision #488 says agents lose the person's admin token.
PR #467 adds approval through GitHub's device flow.
PR #501 adds batched approval by comment.

## Record

<!-- BEGIN aayudha:brain:record -->
- Recorded in #488 Decision: Decision: approve agents' human-only steps from any device, as a group, by a comment from a separate human identity, with the device code as fallback.
<!-- END aayudha:brain:record -->
