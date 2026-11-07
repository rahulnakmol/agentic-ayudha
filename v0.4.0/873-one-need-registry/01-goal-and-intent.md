# Goal and intent: one Need registry

No intent baseline exists (`specs/873/` is absent on `origin/ledger`). The source is the #873 body and its
Initiative.

## Goal

"The inbox kinds move into one Need registry with an area, a seat, a door, a due rule, wake rules, actions, a page
key, a question, and a next action." (#873 Summary). The Initiative's idea: "Every step that needs a person becomes
one record that aayu can find, show, and act on, on every surface" (#870 Summary).

## Who it serves and the outcome

- The person: nothing waits only in a page. "A page can be the only place where something waits." (#873 Problem).
- Later Features that add Need kinds: "Out: the new kinds, which later Features add." (#873 Scope). #880 adds the
  `ask` and `ask-provisional` kinds (#880 Scope); #891 adds a BEHAVIOUR CHANGE card in Decide (#891 Scope).

## Problem

"Ten kinds are hard-coded (`internal/cli/inbox.go:45-56`), and ten page call sites use free-form keys that no inbox
item knows (G1)." (#873 Problem). On `dev` at bc78349 there are 18 kind constants (`internal/cli/inbox.go:46-63`; 17 at 884d81b) and
11 `notify.Page{…}` call sites with keys such as `build:<repo>#<n>`, `halt:<repo>`, and `budget:<scope>` (see
[03-technical-design.md](03-technical-design.md)).

## Success measures

The issue names no measure. A lint that "fails on a page with no kind" (#873 Scope) is a check, so its pass is
binary: zero pages with no kind (inferred).

## Out of scope

- "the new kinds, which later Features add" (#873 Scope, Out).
- Deferred-page delivery itself: it is Bug #859, step S1 (#870 Summary: "S1 is Bug #859").
