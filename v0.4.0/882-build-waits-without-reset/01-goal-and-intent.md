# Goal and intent: a build waits on a person without a reset

No intent baseline exists (`specs/882/` is absent on `origin/ledger`). The source is the #882 body.

## Goal

"A build that asks keeps its lease and budget, and its wall clock pauses until the answer comes." (#882 Summary).

## Who it serves and the outcome

- The builder agent: a question to a person does not cost it its lease, its attempts, or its wall-clock budget.
- The person: an answer lets the same build go on, with no new budget to approve (inferred from "no reset after a
  clarifying answer", #882 Scope).

## Problem

"A blocked build counts its wait on the wall clock, and the next `build` resets the clock and attempts
(`internal/cli/build.go:280-284`) (G11)." (#882 Problem). On `dev` at 884d81b the code has moved to
`internal/cli/build.go:337-344`: "A human made the story Ready again after the block: the budget starts over." The
new state keeps the checkpoints, review spend, review PR, loop, and claim, and drops `StartedAt`, `Attempts`, and
`Failures`. The wall clock is `now - StartedAt` (`internal/cli/build.go:376-377`), and the budget is
`[autonomy.budget] wall_hours = 8` (`internal/config/defaults.toml:75-76`).

## Success measures

None is named in the issue.

## Out of scope

"'Change the story', which keeps today's reset." (#882 Scope, Out).
