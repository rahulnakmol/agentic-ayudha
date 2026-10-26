# Refusals

Questions about another person's records, and requests for medical or financial advice, are refused before any
retrieval, with a message that says who can help.

```feature
{"id": "refusals", "layers": ["L01", "L04"],
 "sources": ["examples/benefits-qa/web/web.go", "examples/benefits-qa/policy/policy.go"],
 "verified_at": "45a18e746769649c356d4dc95b92be3bf83e473d"}
```

## Sub-features

- `personal`: another person's salary, claims, or records are refused.
- `advice`: "should I choose…" and dosage questions are refused.

## How to reach it

- Open `/`, type the question, choose "Ask".

## Behaviour checks

```check
{"id": "personal", "kind": "ui", "evidence": "screenshot", "steps": [
  {"goto": "/"},
  {"fill": {"label": "Your question", "value": "What is my manager's salary?"}},
  {"click": {"role": "button", "name": "Ask"}},
  {"expect": {"role": "heading", "name": "We cannot answer this"}},
  {"expectText": "another person's records"},
  {"screenshot": "personal.png"},
  {"api": {"path": "/api/ask", "json": {"question": "What is my manager's salary?"},
           "status": 200, "body": {"status": "refused", "reason": "personal-data"}}}
]}
```

```check
{"id": "advice", "kind": "ui", "evidence": "screenshot", "steps": [
  {"goto": "/?q=Should%20I%20choose%20the%20HSA%20plan%3F"},
  {"expect": {"role": "heading", "name": "We cannot answer this"}},
  {"expectText": "medical or financial advice"},
  {"screenshot": "advice.png"},
  {"api": {"path": "/api/ask", "json": {"question": "Should I choose the HSA plan?"},
           "status": 200, "body": {"status": "refused", "reason": "advice"}}},
  {"api": {"path": "/api/ask", "json": {"question": "What dosage of ibuprofen can I take?"},
           "status": 200, "body": {"status": "refused", "reason": "advice"}}}
]}
```

## Gotchas

- "my plan's deductible" is not a person's record: possessives of the plan, the employer, or a period are ignored
  before the third-party check. The Ask API feature keeps a non-refusal control.
