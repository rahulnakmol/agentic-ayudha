# #884 Goal and intent

No intent baseline exists for this Feature: `git ls-tree origin/ledger specs/` lists no `specs/884`. This page uses
[#884](https://github.com/rahulnakmol/agentic-ayudha/issues/884) and Story
[#939](https://github.com/rahulnakmol/agentic-ayudha/issues/939) (under Feature #975, v0.4.0).

## Goal

Quoted from #884: "A snooze or a consult is recorded on the ledger, so the terminal, the web, the phone, and a second
machine agree."

## Who it serves and the customer outcome

- The person who works the inbox across sessions and devices.
- #939's Summary states the outcome: "As the founder, I want to put a card or an ask away until a time or an event,
  or ask for help, and to see when it comes back, so that I can take a break and nothing waits without my knowing."

## Problem

Quoted from #884: "Skip lives in memory only (`internal/cli/inboxtui.go:433`) (G12)."

On `dev` the line has moved: the map is `state map[string]string` at `internal/cli/inboxtui.go:442`, and the `s` key
sets "skipped" at `:1394-1398`, with the status "Skipped #%d for now; j and k come back to it." Nothing writes it to
disk or the ledger.

## Scope

In, quoted from #884: "`inbox/<login>.jsonl`; wake on time or on a change of the item; a comment on a question
snoozed past due; Ask someone (a mention, or `aayu counsel --post`)."

## Out of scope

Quoted from #884: "the web screens."

## Success measures

The sources state no success measure for this Feature.

## Dependencies

Issue #884 links `depends-on` [#873](https://github.com/rahulnakmol/agentic-ayudha/issues/873) (One Need registry; not
sliced). "A question snoozed past due" also needs questions with a due time, which are Feature #883's
([#883](https://github.com/rahulnakmol/agentic-ayudha/issues/883)) and #880's (inferred; #884 does not link them).
