# Product events

Every page view records a visit, and every question records `question_asked` plus its outcome
(`answer_grounded`, `answer_referred`, or `answer_refused`), keyed by a pseudonymous visitor ID, never the question
text. The Goal's metrics are computed from these events.

```feature
{"id": "product-events", "layers": ["L03", "L08"],
 "sources": ["examples/benefits-qa/ask/ask.go", "examples/benefits-qa/analytics/analytics.go",
             "examples/benefits-qa/cmd/benefits-qa/main.go"],
 "routes": [],
 "verified_at": "30468e38ca8271ea573481703c7c9ae80024f52e"}
```

## Sub-features

- `visit-and-ask`: one visit and one answered question append `benefits_visit`, `question_asked`, and
  `answer_grounded` for the same visitor, with no question text.

## How to get to it (user POV)

- Not visible to the employee. An analyst reads the Application Insights export.

## Driving it with control-bqa

```journey
{"id": "visit-and-ask", "evidence": "api-trace", "requires": "events", "steps": [
  {"goto": "/?q=What%20is%20the%20dental%20annual%20deductible%3F"},
  {"events": {"names": ["benefits_visit", "question_asked", "answer_grounded"], "noText": "dental"}}
]}
```

## Gotchas

- `deploy/serve.sh` starts the app without `-events`, so this journey is unreachable through the `[verify] seed`
  launch. Until the seed passes `-events`, its proof is the Go test evidence (`analytics_test.go`), not a drive.
