# Ask API

`POST /api/ask` takes `{"question", "user_id"}` and answers with the same outcome the page shows. Requests outside
the contract (`contracts/ask.schema.json`) are refused with a 4xx and a JSON error. `GET /healthz` names the
instance that answers.

```feature
{"id": "api-ask", "layers": ["L02", "L06"],
 "sources": ["examples/benefits-qa/api/api.go", "examples/benefits-qa/contracts/contracts.go",
             "examples/benefits-qa/contracts/ask.schema.json"],
 "surfaces": {"route": ["/api/ask", "GET /healthz"]},
 "verified_at": "aef4fe09c450db455388128e1359f5fb0fc7a561"}
```

## Sub-features

- `answered`: a plan question returns `status: answered` with citations; "my plan's" is not another person's record.
- `errors`: wrong method (405), wrong media type (415), unknown field (400), empty question (400).
- `health`: `GET /healthz` answers 200.

## How to reach it

- A client posts JSON to `/api/ask` (the integration surface; the page does not call it).

## Behaviour checks

```check
{"id": "answered", "kind": "api", "evidence": "api-trace", "steps": [
  {"api": {"path": "/api/ask", "json": {"question": "Does Example Co match my 401(k) contributions?", "user_id": "v-verify"},
           "status": 200, "body": {"status": "answered"}}},
  {"api": {"path": "/api/ask", "json": {"question": "What is my plan's dental deductible?"},
           "status": 200, "body": {"status": "answered"}}}
]}
```

```check
{"id": "errors", "kind": "api", "evidence": "api-trace", "steps": [
  {"api": {"method": "GET", "path": "/api/ask", "status": 405}},
  {"api": {"path": "/api/ask", "raw": "question=hi", "contentType": "text/plain", "status": 415}},
  {"api": {"path": "/api/ask", "json": {"question": "hi", "name": "Ada"}, "status": 400}},
  {"api": {"path": "/api/ask", "json": {"question": "  "}, "status": 400}}
]}
```

```check
{"id": "health", "kind": "api", "evidence": "api-trace", "steps": [
  {"api": {"method": "GET", "path": "/healthz", "status": 200}}
]}
```

## Gotchas

- Keys are case-sensitive and may appear once; `{"Question": …}` is an unknown field, not a question.
