# Referral to HR

A question the plan documents do not answer is not guessed at: the employee is told an HR specialist will reply,
and no passage is quoted.

```feature
{"id": "refer-to-hr", "layers": ["L01", "L04"],
 "sources": ["examples/benefits-qa/web/web.go", "examples/benefits-qa/policy/policy.go"],
 "routes": [],
 "verified_at": "30468e38ca8271ea573481703c7c9ae80024f52e"}
```

## Sub-features

- `typed`: an off-plan question shows "We sent this to HR" with the reply time and no Source line.

## How to get to it (user POV)

- Open `/`, type a question the plans do not cover, choose "Get answer".

## Driving it with control-bqa

```journey
{"id": "typed", "evidence": "screenshot", "steps": [
  {"goto": "/"},
  {"fill": {"label": "Your question", "value": "How do I adopt a pet?"}},
  {"click": {"role": "button", "name": "Get answer"}},
  {"expect": {"role": "heading", "name": "We sent this to HR"}},
  {"expectText": "within 3 business days"},
  {"absentText": "Source: "},
  {"screenshot": "referred.png"},
  {"api": {"path": "/api/ask", "json": {"question": "How do I adopt a pet?", "user_id": "v-verify"},
           "status": 200, "body": {"status": "referred", "reason": "not-in-plan-documents"}}}
]}
```

## Gotchas

- Nothing is actually sent to HR in this example; the referral is a message and a product event only.
