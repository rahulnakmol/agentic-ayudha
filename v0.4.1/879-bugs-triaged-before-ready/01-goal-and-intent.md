# #879 Goal and intent

No intent baseline exists for this Feature: `git ls-tree origin/ledger specs/` lists no `specs/879`. This page uses
[#879](https://github.com/rahulnakmol/agentic-ayudha/issues/879), Decision
[#861](https://github.com/rahulnakmol/agentic-ayudha/issues/861), and Story
[#954](https://github.com/rahulnakmol/agentic-ayudha/issues/954) (under Feature #975, v0.4.0).

## Goal

Quoted from #879: "`aayu triage <bug> --decide hotfix|feature|repro|not-a-bug` (human-only) records how a Bug is
worked, and Ready refuses a Bug with no hotfix triage."

## Who it serves and the customer outcome

- The founder: #954 says "I want a TRIAGE card with the repro report and counsel, so that I decide how each bug is
  worked."
- Decision #861 states the reason: "The founder wants a person to agree that a Feature is what is needed before any
  of its stories are built." For a Bug, the triage decision is that agreement: "A Bug avoids this through triage: a
  hotfix needs no Feature."

## Problem

Quoted from #879: "`aayu triage` only sets labels today (Q17, C29). Decision #861."

The code confirms it on `dev` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b): the triage
package "classifies an intake issue as Feedback, Bug, or Incident and records the result in labels only"
(`internal/triage/triage.go`, package comment). In `aayu ready`, the `Covered` hook returns no rule for any type but
`story` (`internal/cli/ready.go:150-158`).

## Scope

In, quoted from #879: "a "Where it happens" field on the Bug form; `specs/<bug>/triage.json` with the repro hash and
the hotfix path (ordinary or emergency); the TRIAGE Need kind; the Ready rule."

## Out of scope

Quoted from #879: "the emergency release steps, which CONTRIBUTING.md keeps."

Decision #861 adds: "The new rules apply at Ready only. The Ready guard runs `Validate` again on every edit, so it
must not apply them, or older Ready stories lose their label (core design F4)." And: "Triage never skips a gate."

## Success measures

The sources state no success measure. Decision #861 names a review trigger: "if one-story Features become common,
allow a small Feature with only the intent and spec baselines."
