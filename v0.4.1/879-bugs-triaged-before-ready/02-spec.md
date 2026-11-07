# #879 Spec

No spec baseline exists (`specs/879` is absent from the ledger) and no Story is filed under #879. Requirements trace
to #879, Decision #861 option 1 (accepted), and Story #954's Completion Goal (under Feature #975). Slices are
proposed, not filed (see [05-implementation-plan.md](05-implementation-plan.md)).

## Functional requirements

| # | Requirement | Source | Slice |
|---|---|---|---|
| FR1 | The Bug form gets a field "Where it happens". The existing "Environment" field keeps its meaning (where the proof is captured). | [#879](https://github.com/rahulnakmol/agentic-ayudha/issues/879) Scope; [#861](https://github.com/rahulnakmol/agentic-ayudha/issues/861) option 1 | P1 |
| FR2 | `aayu triage <bug> --decide hotfix\|feature\|repro\|not-a-bug` is human-only and records the decision. | #879 Summary; #861 option 1 | P2 |
| FR3 | The decision is written to `specs/<bug>/triage.json` with the hash of the repro report and the hotfix path, ordinary or emergency. | #879 Scope; #861 option 1 | P2 |
| FR4 | `not-a-bug` takes a one-line reason. | [#954](https://github.com/rahulnakmol/agentic-ayudha/issues/954) criterion 4 (`--reason <reason>`) | P2 |
| FR5 | `aayu ready <bug>` refuses a Bug with no hotfix triage. | #879 Summary; #861 option 1 | P3 |
| FR6 | The rule runs at Ready only, never in the Ready guard. | #861 Evaluation | P3 |
| FR7 | A TRIAGE Need kind shows the repro fields, the counsel, and "1 Hotfix · 2 Carve out a Feature · 3 Need a better repro · 4 Not a bug". | #879 Scope; #954 criterion 1 | P4 |
| FR8 | With "Where it happens" empty, Hotfix and Carve out are off, and 3 asks the reporter. | #954 criterion 2 | P4 |
| FR9 | Choice 1 runs one presence check, then `aayu triage <n> --decide hotfix` as the person. | #954 criterion 3 | P2, P4 |

## Non-functional requirements

| Kind | Requirement | Source |
|---|---|---|
| Security | The triage decision is a person's (human-only, presence check). Today `triage` is `IdentityBot` with `GateLabelsOnly`; the new flag changes that. | #879 Summary; `internal/verbs/registry.go`, entry `Name: "triage"` |
| Security | The issue text reaches a model only inside a delimited provenance block. | `triage.Untrusted` (`internal/triage/triage.go:44`) |
| Reliability | Older Ready items keep their label: the guard does not apply the new rule. | #861 Evaluation (F4) |
| Process | A hotfix keeps the CONTRIBUTING.md paths: an ordinary PR to `dev`, or the emergency path, which still merges into `dev` first. | #861 Evaluation |

## Open questions

- What do `feature` and `repro` do beyond the record: file a Feature, ask the reporter? #861 and #954 describe the
  choices, not their writes. #954 says only: "Posting the reason and closing the bug are #879's (S3)" (#954
  Assumptions).
- Does a Bug inside a mission's BUGS milestone (PR #1216) also need triage before Ready? Unverified.
