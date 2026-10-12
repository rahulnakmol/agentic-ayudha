# rahulnakmol/agentic-ayudha#126: Self-serve benefits answers quoted from the plan documents

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, initiative |
| Body hash | `c99137b427d7ca35e442364d1fa3bcaa8b2afa0047835d90cace96654cfb7aea` |
| Completion Goal hash | n/a (no Completion Goal on this type) |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/126/ready.md` |

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

Self-serve benefits answers: an Ask page and API that quote the Example Co plan documents, refuse what must not be answered, and refer the rest to HR. Serves Goal #125.

### Hypothesis

If employees can ask the plan documents directly and get a quoted, cited answer at once, activation (an answer within 7 days of the first visit) rises from 0.258 to at least 0.60, because most benefits questions are answered word for word in the plan documents and today wait in the HR ticket queue. Riskiest assumption: that the questions employees ask are covered by the plan text closely enough for retrieval to ground them (the synthetic question set models this; a real launch would test it).

### Metric

activation

### Baseline

0.258 (31 of 120 synthetic employees), the Goal #125 baseline: `aayu measure --appinsights examples/benefits-qa/analytics/testdata/hr-ticket-seed1-2026-08-10.jsonl --metric activation --from 2026-08-10 --to 2026-09-07`, snapshot SHA-256 `81dafa37e0cb9b1950babad7be35aa459331a8011847ef8f3ddc691f80a82018`.

### Target

>= 0.60 activation on the product channel's synthetic export over the 28 days after the example ships
````
