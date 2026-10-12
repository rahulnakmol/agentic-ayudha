# rahulnakmol/agentic-ayudha#127: Grounded answers API (benefits-qa)

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, feature |
| Body hash | `4f449129d282ef3477292084f72a06ffaae1f1e62d07f7a899c5b46653b5ed2e` |
| Completion Goal hash | n/a (no Completion Goal on this type) |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/127/ready.md` |

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

Grounded answers: a question about Example Co benefits gets the plan passage that answers it, quoted with its plan and section, through `POST /api/ask`; questions about another person's records or asking for advice are refused, and questions the documents do not answer are referred to HR. Serves Initiative #126.

### Problem

Before the product every benefits question is an HR ticket: in the synthetic baseline only 0.258 of employees get an answer within 7 days of first asking (Goal #125), although most questions are answered word for word in the plan documents.

### Scope

In scope: the synthetic plan documents of Example Co (data), retrieval of the best passage with a grounding score, the answer and refusal policy, the API contract (JSON Schema) and its HTTP handler, and the question workflow that ties them together and emits product events. Out of scope: any language model, real plan data, authentication, and the web page (Feature: Ask page).

### Expected paths

examples/benefits-qa/store/**
examples/benefits-qa/policy/**
examples/benefits-qa/answer/**
examples/benefits-qa/contracts/**
examples/benefits-qa/ask/**
examples/benefits-qa/api/**
````
