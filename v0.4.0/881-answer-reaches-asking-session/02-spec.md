# Spec: the answer reaches the asking session

No spec baseline and no Story exist. Each requirement is traced to the #881 body and to a proposed slice in
[05-implementation-plan.md](05-implementation-plan.md). Proposed slices are not filed.

## Functional requirements

| # | Requirement | Source | Proposed slice |
|---|---|---|---|
| FR1 | Each delivery has a receipt: sent, pull-pending, read, or failed. | #881 Scope | P1 |
| FR2 | An answer to a cloud session is pushed through the Amp, Claude cloud, Codex cloud, or Cursor adapter. | #881 Scope | P2 |
| FR3 | A local session gets its answer through `aayu ask --wait`. | #881 Summary and Scope | P3 |
| FR4 | Hosts learn the flow: `skills/ask/SKILL.md` and the host files (inferred from #881 Expected paths `skills/ask/SKILL.md`, `hosts/**`, `internal/hosts/**`, `internal/cli/install.go`). | inferred | P4 |
| FR5 | The inbox shows the receipt (inferred from #881 Expected paths `internal/cli/inbox.go`). | inferred | P5 |

## Non-functional requirements

| Area | Requirement | Source |
|---|---|---|
| Reliability | A receiver call is bounded: `receiverTimeout` is 2 minutes, and a stalled CLI is a failed call (`internal/cli/kanha.go:134-147`). Inferred: an answer push uses the same bound, and a timeout gives the receipt `failed`. | code; inferred |
| Security | Answer text is untrusted and is quoted as data (ETHOS.md principle 6). Inferred for the pushed message. | inferred |
| Operability | Every send is an external write in the actions log, as `mission.kanha.tell` is today (`internal/cli/kanha.go:165`). Inferred for an answer push. | code; inferred |
