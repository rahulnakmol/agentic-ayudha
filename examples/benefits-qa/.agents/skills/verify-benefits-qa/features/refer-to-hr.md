# Referral to HR

A question the plan documents do not answer is not guessed at: the employee is told an HR specialist will reply,
and no passage is quoted.

```feature
{"id": "refer-to-hr", "layers": ["L01", "L04"],
 "sources": ["examples/benefits-qa/web/web.go", "examples/benefits-qa/policy/policy.go"],
 "verified_at": "aef4fe09c450db455388128e1359f5fb0fc7a561"}
```

## Sub-features

- `typed`: an off-plan question shows "We sent this to HR" with the reply time and no Source line.

## How to reach it

- Open `/`, type a question the plans do not cover, choose "Ask".

## Behaviour checks

```check
{"id": "typed", "kind": "ui", "evidence": "screenshot", "steps": [
  {"goto": "/"},
  {"fill": {"label": "Your question", "value": "How do I adopt a pet?"}},
  {"click": {"role": "button", "name": "Ask"}},
  {"expect": {"role": "heading", "name": "We sent this to HR"}},
  {"expectText": "within 2 business days"},
  {"absentText": "Source: "},
  {"screenshot": "referred.png"},
  {"api": {"path": "/api/ask", "json": {"question": "How do I adopt a pet?", "user_id": "v-verify"},
           "status": 200, "body": {"status": "referred", "reason": "not-in-plan-documents"}}}
]}
```

## Gotchas

- Nothing is sent to HR in this example; the referral is a message and a product event only.
