# Refusals

Questions about another person's records, and requests for medical or financial advice, are refused before any
retrieval, with a message that says who can help.

```feature
{"id": "refusals", "layers": ["L01", "L04"],
 "sources": ["examples/benefits-qa/web/web.go", "examples/benefits-qa/policy/policy.go"],
 "routes": [],
 "verified_at": "86275c2423cd8c0a6c61ab996ba560129deb4c2a"}
```

## Sub-features

- `personal`: another person's salary, claims, or records are refused.
- `advice`: "should I choose…" and dosage questions are refused.

## How to get to it (user POV)

- Open `/`, type the question, choose "Ask".

## Driving it with control-bqa

```journey
{"id": "personal", "evidence": "screenshot", "steps": [
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

```journey
{"id": "advice", "evidence": "screenshot", "steps": [
  {"goto": "/?q=Should%20I%20choose%20the%20HSA%20plan%3F"},
  {"expect": {"role": "heading", "name": "We cannot answer this"}},
  {"expectText": "medical or financial advice"},
  {"screenshot": "advice.png"}
]}
```

## Gotchas

- "my plan's deductible" is not a person's record: possessives of the plan, the employer, or a period are ignored
  before the third-party check. Keep a non-refusal control in the API feature.
