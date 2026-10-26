# Ask API

`POST /api/ask` takes `{"question", "user_id"}` and answers with the same outcome the page shows. Requests outside
the contract (`contracts/ask.schema.json`) are refused with a 4xx and a JSON error. `GET /healthz` names the
instance that answers.

```feature
{"id": "api-ask", "layers": ["L02", "L06"],
 "sources": ["examples/benefits-qa/api/api.go", "examples/benefits-qa/contracts/contracts.go",
             "examples/benefits-qa/contracts/ask.schema.json"],
 "surfaces": {"route": ["/api/ask", "GET /healthz"]},
 "verified_at": "45a18e746769649c356d4dc95b92be3bf83e473d"}
```

## Sub-features

- `answered`: a plan question returns `status: answered` with citations; "my plan's" is not another person's record.
- `errors`: wrong method (405), wrong media type (415), and 400 for an unknown field, an other-case or duplicate
  key, a `null` value, text after the object, a `user_id` that is not pseudonymous, and an empty question. Each
  carries its JSON error.
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
  {"api": {"method": "GET", "path": "/api/ask", "status": 405, "body": {"error": "use POST with a JSON body"}}},
  {"api": {"path": "/api/ask", "raw": "question=hi", "contentType": "text/plain", "status": 415,
           "body": {"error": "send Content-Type: application/json"}}},
  {"api": {"path": "/api/ask", "json": {"question": "hi", "name": "Ada"}, "status": 400,
           "body": {"error": "the body is not an AskRequest: unknown field \"name\""}}},
  {"api": {"path": "/api/ask", "json": {"Question": "hi"}, "status": 400,
           "body": {"error": "the body is not an AskRequest: unknown field \"Question\""}}},
  {"api": {"path": "/api/ask", "raw": "{\"question\": \"dental\", \"question\": \"vision\"}", "status": 400,
           "body": {"error": "the body is not an AskRequest: duplicate field \"question\""}}},
  {"api": {"path": "/api/ask", "raw": "{\"question\": null}", "status": 400,
           "body": {"error": "the body is not an AskRequest: field \"question\": want a string"}}},
  {"api": {"path": "/api/ask", "raw": "{\"question\": \"dental\"} {}", "status": 400,
           "body": {"error": "the body is not an AskRequest: something follows the JSON object"}}},
  {"api": {"path": "/api/ask", "json": {"question": "hi", "user_id": "ada@example.com"}, "status": 400,
           "body": {"error": "user_id must be a pseudonymous ID of letters, digits, and hyphens"}}},
  {"api": {"path": "/api/ask", "json": {"question": "  "}, "status": 400, "body": {"error": "question is required"}}}
]}
```

```check
{"id": "health", "kind": "api", "evidence": "api-trace", "steps": [
  {"api": {"method": "GET", "path": "/healthz", "status": 200}}
]}
```

## Gotchas

- Keys are case-sensitive and may appear once; `{"Question": …}` is an unknown field, not a question.
- A body over 4 KiB (413) and a question over 500 characters (400) are not driven here: a step would have to
  spell the long body out. `TestAskRejects` (`api/api_test.go`) covers both.
