# Goal and intent: aayu ask

No intent baseline exists (`specs/880/` is absent on `origin/ledger`). The source is the #880 body.

## Goal

"A new `aayu ask` verb posts one structured question on an item, and a person answers it in the inbox, in the agent
tool (two-way only), or by an `/aayu answer` reply." (#880 Summary).

## Who it serves and the outcome

- Every seat's agent: "Any seat asks" (#880 title). A builder that needs a person asks one structured question
  instead of stopping the build.
- The person: one place to answer, on any of three surfaces, and the first verified answer settles it (#880 Scope).
- Behaviours: Decision #867 says "A builder that thinks a behaviour is wrong runs `aayu ask`. It never edits a check to
  make the gate green" (#867 option 1).

## Problem

"Only Decision issues and `status:blocked` with free text exist (`internal/cli/build.go:1110-1135`) (G2, Q10, Q14,
E3)." (#880 Problem). On `dev` at 884d81b the same holds: `aayu build --stop` moves a story to `status:blocked` "with
this reason and page its owner" (`aayu build --help`; `Env.markBlocked`, `internal/cli/build.go:1208`), and a person
runs `aayu ready <n>` to let it continue "with a fresh budget" (`Env.blockBuild`, `internal/cli/build.go:1198`).

## Success measures

The issue names no measure. The Initiative's metric is PRs merged into dev (#870 Metric). The evidence does not show
how many builds stop today for a question.

## Out of scope

- "delivery to the session (S12)" (#880 Scope, Out). That is Feature #881.
- The SLA and the provisional answer at 48 h: Decision #862 (accepted) sets them; Feature #883 ("Questions have an SLA,
  and overdue ones come first", milestone v0.4.1) builds them (`/tmp/rm/issues-v0.4.1.jsonl`). #883 Links say
  "depends-on #880"; #880 Links do not name #883.
