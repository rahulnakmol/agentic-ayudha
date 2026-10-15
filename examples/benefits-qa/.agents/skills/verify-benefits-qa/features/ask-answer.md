# Ask and get an answer

An employee types a benefits question, or picks a suggested one, and reads the plan passage that answers it, quoted,
with the plan and section it came from.

```feature
{"id": "ask-answer", "layers": ["L01", "L03", "L05"],
 "sources": ["examples/benefits-qa/web/web.go", "examples/benefits-qa/web/static/app.css",
             "examples/benefits-qa/ask/ask.go", "examples/benefits-qa/answer/answer.go",
             "examples/benefits-qa/store/seed/plans.json"],
 "routes": ["GET /{$}", "GET /app.css", "GET /tokens.css"],
 "verified_at": "30468e38ca8271ea573481703c7c9ae80024f52e"}
```

## Sub-features

- `typed`: a typed question shows an Answer card with the quote and its Source line.
- `suggestion`: a suggested question on the empty page answers in one click.
- `empty`: the empty page offers three suggested questions.

## How to get to it (user POV)

- Open `/`, type in "Your question", choose "Get answer".
- Open `/`, choose a suggested question under "Try a question".
- Open `/?q=<question>` directly (every state has an address).

## Driving it with control-bqa

```journey
{"id": "typed", "evidence": "recording", "steps": [
  {"goto": "/"},
  {"expect": {"role": "heading", "name": "Ask about your benefits"}},
  {"fill": {"label": "Your question", "value": "What is the dental annual deductible?"}},
  {"click": {"role": "button", "name": "Get answer"}},
  {"expect": {"role": "heading", "name": "Answer"}},
  {"expectText": "50 dollars per person"},
  {"expectText": "Source: Dental, Deductible"},
  {"screenshot": "answer.png"},
  {"api": {"path": "/api/ask", "json": {"question": "What is the dental annual deductible?", "user_id": "v-verify"},
           "status": 200, "body": {"status": "answered"}}}
]}
```

```journey
{"id": "suggestion", "evidence": "screenshot", "steps": [
  {"goto": "/"},
  {"click": {"role": "link", "name": "How many weeks of parental leave do I get?"}},
  {"expect": {"role": "heading", "name": "Answer"}},
  {"expectText": "Source: "},
  {"screenshot": "suggestion.png"}
]}
```

```journey
{"id": "empty", "evidence": "screenshot", "steps": [
  {"goto": "/"},
  {"expect": {"role": "heading", "name": "Try a question"}},
  {"count": {"role": "link", "within": "Try a question", "equals": 3}},
  {"screenshot": "empty.png"}
]}
```

## Gotchas

- The page sets a `bqa_uid` cookie on first visit; a journey that compares two visits must keep the same context.
- The Ask button submits a GET form, so the answer state is a new page load at `/?q=…`, not an in-page update.
- An answer is shown only when the cited passage covers at least half the question's terms (`policy.MinScore`); a
  rephrased question can turn an answer into a referral without any code change.
