# Ask API

`POST /api/ask` takes `{"question", "user_id"}` and answers with the same outcome the page shows. Requests outside
the contract (`contracts/ask.schema.json`) are refused with a 4xx and a JSON error.

```feature
{"id": "api-ask", "layers": ["L02", "L06"],
 "sources": ["examples/benefits-qa/api/api.go", "examples/benefits-qa/contracts/contracts.go",
             "examples/benefits-qa/contracts/ask.schema.json"],
 "routes": ["/api/ask", "GET /healthz"],
 "verified_at": "86275c2423cd8c0a6c61ab996ba560129deb4c2a"}
```

## Sub-features

- `answered`: a plan question returns `status: answered` with citations.
- `errors`: wrong method (405), wrong media type (415), unknown field (400), empty question (400).

## How to get to it (user POV)

- A client posts JSON to `/api/ask` (the integration surface; the page does not call it).

## Driving it with control-bqa

```journey
{"id": "answered", "evidence": "api-trace", "steps": [
  {"api": {"path": "/api/ask", "json": {"question": "Does Example Co match my 401(k) contributions?", "user_id": "v-verify"},
           "status": 200, "body": {"status": "answered"}}}
]}
```

```journey
{"id": "errors", "evidence": "api-trace", "steps": [
  {"api": {"method": "GET", "path": "/api/ask", "status": 405}},
  {"api": {"path": "/api/ask", "raw": "question=hi", "contentType": "text/plain", "status": 415}},
  {"api": {"path": "/api/ask", "json": {"question": "hi", "name": "Ada"}, "status": 400}},
  {"api": {"path": "/api/ask", "json": {"question": "  "}, "status": 400}}
]}
```

## Gotchas

- Keys are case-sensitive and may appear once; `{"Question": …}` is an unknown field, not a question.
