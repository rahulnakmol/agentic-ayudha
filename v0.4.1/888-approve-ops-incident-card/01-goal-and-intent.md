# #888 Goal and intent

No intent baseline exists for this Feature: `git ls-tree origin/ledger specs/` lists no `specs/888`. This page uses
[#888](https://github.com/rahulnakmol/agentic-ayudha/issues/888) and the related Story
[#947](https://github.com/rahulnakmol/agentic-ayudha/issues/947) (under Feature #975, v0.4.0).

## Goal

Quoted from #888: "`aayu respond <incident> --approve <run>` (human-only, one-way) approves a pending ops deployment
through GitHub's API, as the person."

## Who it serves and the customer outcome

- The person on call in an incident, who today must leave the terminal for GitHub's Actions page (#888 Problem).
- #947's Summary states the outcome: "As the founder, I want an INCIDENT card that shows the proposed action, its
  blast radius and rollback, and approves it through GitHub as me, so that I act in an incident from the command
  center."

## Problem

Quoted from #888: "An ops action can be approved only on GitHub's Actions page (`internal/cli/respond.go:395-411`)
(Q12)."

On `dev`, those lines are `humanApproval`, which reads the run's approvals after the fact. It does not approve
anything. `propose` dispatches `aayudha-ops.yml` and the output says "awaiting a human reviewer's approval of
environment … : `<run URL>`" (`internal/cli/respond.go:226-228`).

## Scope

In, quoted from #888: "`requireReviewers` first, unchanged; the pending-deployments call after the presence check;
the incident-action Need kind in Now; a warning when the environment has prevent self-review (F8). Its PR needs a
person to ship."

## Out of scope

Quoted from #888: "any second way into production." GitHub's environment protection stays the gate: the approval
is the person's own, through GitHub's API.

## Success measures

The sources state no success measure for this Feature.
