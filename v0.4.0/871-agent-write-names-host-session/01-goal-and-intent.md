# Goal and intent: host and session on every agent write

No intent baseline exists: `git ls-tree origin/ledger specs/871` is empty. The source is the #871 body.

## Goal

"aayu records the host, the session ID, and the session URL on every agent write: actions records, build state,
asks, reviews, and drafts." (#871 Summary). It is step S2 of the core design, "agreed by @rahulnakmol on
2026-10-21" (#871 Summary; Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870)).

## Who it serves and the outcome

- The person who works the inbox: "the inbox cannot link a card to the thread it came from (G4)" (#871 Problem). The
  outcome is a card that links to the agent thread that caused it.
- The Features that build on it: #872 records host and model per merged story, and #880 asks need a session to
  answer to (both list `depends-on #871`).

## Problem

From the #871 body, checked on `dev` at 884d81b:

1. "Today the session is read only for the asked gate (`internal/cli/asked.go:37-44`)." The function is
   `Env.session()` at `internal/cli/asked.go:37`. Since the issue was filed, a second reader exists: a baseline draft
   records `SourceSession: r.e.session()` (`internal/cli/ready.go:528`).
2. "`Action.Host` is empty in a normal run (`internal/cli/actions.go:85`)." Still true: `Env.Actions()` passes
   `Host: e.Host` (`internal/cli/actions.go:85`), and `e.Host` is the `--host` flag, empty by default
   (`internal/cli/root.go:126`).
3. "The build state has no host or session." Still true: `BuildState` has no such field
   (`internal/cli/build.go:99`).

## Success measures

The issue names no measure or threshold. The Initiative's metric is PRs merged into dev, baseline 527 in 28 days,
target 1,000 (#870 Metric, Baseline, Target). The evidence does not show how #871 moves it.

## Out of scope

- "using the source as authority; it is information only" (#871 Scope, Out). `ready.Parent` already carries the
  same rule in its comment: "It is information, never authority (#871)" (`internal/ready/approve.go:44`).
