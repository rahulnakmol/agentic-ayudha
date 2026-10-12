# rahulnakmol/agentic-ayudha#132: POST /api/ask answers within its JSON Schema contract and records events

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `17db0c69d847275a4a59aba81fd66b07b953b0e5e03a9e2fc9a4c909bc32683c` |
| Completion Goal hash | `9c89e7daa33ab821a3f9e9627735e7616e101f821dea16568ce812aedc617c79` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/132/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: POST /api/ask with {"question":"What is the dental annual deductible?"}; expected: 200 with status answered, the quoted passage, and its citation, in a body that decodes strictly as the contract's AskResponse
2. [api-trace] state: POST /api/ask with malformed JSON, an empty question, an unknown field, an email address as user_id, a form body, or a body over 4 KiB, and GET /api/ask; expected: a 4xx with a JSON error body for each
3. [api-trace] state: the ask workflow answers, refers, and refuses one question each for user emp-0007; expected: it records question_asked and then exactly one of answer_grounded, answer_referred, or answer_refused for that user, at the service clock in UTC
4. [api-trace] state: the committed `contracts/ask.schema.json` and the Go contract types; expected: the schema generated from the types equals the committed file byte for byte

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As a client of the benefits-qa example, I want POST /api/ask to answer within a published JSON Schema contract and the workflow to record what happened, so that I can integrate against it and the Goal can be measured.

### Completion Goal

1. [api-trace] state: POST /api/ask with {"question":"What is the dental annual deductible?"}; expected: 200 with status answered, the quoted passage, and its citation, in a body that decodes strictly as the contract's AskResponse
2. [api-trace] state: POST /api/ask with malformed JSON, an empty question, an unknown field, an email address as user_id, a form body, or a body over 4 KiB, and GET /api/ask; expected: a 4xx with a JSON error body for each
3. [api-trace] state: the ask workflow answers, refers, and refuses one question each for user emp-0007; expected: it records question_asked and then exactly one of answer_grounded, answer_referred, or answer_refused for that user, at the service clock in UTC
4. [api-trace] state: the committed `contracts/ask.schema.json` and the Go contract types; expected: the schema generated from the types equals the committed file byte for byte

### Measure

Completion Goal criteria observed

### Threshold

all 4 criteria

### Scenario

Call POST /api/ask on a test server with a covered question and with each malformed request; run the workflow with an in-memory recorder; regenerate the schema.

### Environment

local checkout with the embedded synthetic Example Co seed; synthetic data only (V3), never production

### Links

depends-on #131
````
