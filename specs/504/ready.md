# rahulnakmol/agentic-ayudha#504: API developer-experience floors: no breaking contract change without a waiver, and default p95 and error budgets

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `7a52b3ed32cb7e2f8fc50985a64cfa7c3b2218fff9cb7e98885e92dbc7a9c47f` |
| Completion Goal hash | `0509216808fa560a502f91cfde2b89fc54df681b79e114175908fae60d29852c` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/504/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: an OpenAPI spec whose head removes a required response field; then one whose head adds an optional response field; then a pull request that removes the field and also edits the base version of the spec in the same commit range; expected: the removal fails the contract floor naming the operation and the field; the optional field passes; the third still fails because the spec is compared from the base commit, not from the edited copy
2. [api-trace] state: a `.proto` file whose head renumbers a field, with and without a `buf.yaml` that sets breaking rules; and a GraphQL schema whose head removes a field of a type; expected: both fail the contract floor; buf uses the repository's rules when `buf.yaml` is present and `FILE` otherwise, and the check records which ran
3. [api-trace] state: `[verify] platforms = ["api"]`, a footprint with L02 and no spec; then with a breaking change and an accepted floor-waiver Decision that names contract; then `platforms = ["web"]` with the same change; expected: the first records `skipped: contract`, which blocks ship until a waiver names contract; the waiver releases the break; the third skips without blocking
4. [api-trace] state: a stand-in k6 summary export with `http_req_duration` p(95) of 600 ms, then 400 ms with `http_req_failed` 0.02, then 400 ms with 0.005; and an OpenAPI spec with two GET operations, one with a required parameter, and no k6 script; expected: the first fails naming p95 and 500 ms, the second fails naming the error rate, the third passes; with no script verify generates one that sends 20 requests to the GET operation with no required parameter only; with neither a script nor a spec the check reads `skipped: api` and blocks ship until waived
5. [api-trace] state: a contract or api miss with the clock before 2026-11-14, then on 2026-11-14; expected: before the date the check passes with a detail that says advisory until 2026-11-14 and names the miss; on the date it fails
6. [api-trace] state: the merge gate as `aayu init --gates` renders it, run with stand-ins whose oasdiff, buf and graphql-inspector downloads do not match their pinned SHA-256; expected: the install step fails before the floors run; with matching stand-ins the tools install and the floors run

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As a product team using aayu, I want a story that changes an API to prove it broke no client and stayed fast, so that a removed response field or a slower endpoint cannot merge unnoticed. Decision #480 (accepted, option 1, section 3) adds a `contract` floor that compares each OpenAPI, protobuf and GraphQL spec from the base commit to the head with oasdiff 1.33.0, buf 1.74.0 and @graphql-inspector/core 8.0.1, each pinned by SHA-256, and fails on a breaking change unless a floor waiver names contract. It gives the k6 `api` floor default budgets: p95 500 ms and an error rate of 1 %. Both apply when `[verify] platforms` has `api` and the footprint has L02 or L06. Misses are advisory until `[verify] new_floors_blocking_from` (2026-11-14), then they fail.

### Completion Goal

1. [api-trace] state: an OpenAPI spec whose head removes a required response field; then one whose head adds an optional response field; then a pull request that removes the field and also edits the base version of the spec in the same commit range; expected: the removal fails the contract floor naming the operation and the field; the optional field passes; the third still fails because the spec is compared from the base commit, not from the edited copy
2. [api-trace] state: a `.proto` file whose head renumbers a field, with and without a `buf.yaml` that sets breaking rules; and a GraphQL schema whose head removes a field of a type; expected: both fail the contract floor; buf uses the repository's rules when `buf.yaml` is present and `FILE` otherwise, and the check records which ran
3. [api-trace] state: `[verify] platforms = ["api"]`, a footprint with L02 and no spec; then with a breaking change and an accepted floor-waiver Decision that names contract; then `platforms = ["web"]` with the same change; expected: the first records `skipped: contract`, which blocks ship until a waiver names contract; the waiver releases the break; the third skips without blocking
4. [api-trace] state: a stand-in k6 summary export with `http_req_duration` p(95) of 600 ms, then 400 ms with `http_req_failed` 0.02, then 400 ms with 0.005; and an OpenAPI spec with two GET operations, one with a required parameter, and no k6 script; expected: the first fails naming p95 and 500 ms, the second fails naming the error rate, the third passes; with no script verify generates one that sends 20 requests to the GET operation with no required parameter only; with neither a script nor a spec the check reads `skipped: api` and blocks ship until waived
5. [api-trace] state: a contract or api miss with the clock before 2026-11-14, then on 2026-11-14; expected: before the date the check passes with a detail that says advisory until 2026-11-14 and names the miss; on the date it fails
6. [api-trace] state: the merge gate as `aayu init --gates` renders it, run with stand-ins whose oasdiff, buf and graphql-inspector downloads do not match their pinned SHA-256; expected: the install step fails before the floors run; with matching stand-ins the tools install and the floors run

### Measure

Completion Goal criteria observed

### Threshold

all 6 criteria

### Scenario

Run the contract and api floors against git fixtures of a base and a head commit with recorded specs, stand-in oasdiff, buf and graphql-inspector, and recorded k6 summaries, across the platform, footprint, waiver and date combinations above. Run the merge gate install step with stand-in downloads.

### Environment

local checkout and this repository's CI build job on a pull request; the fake GitHub in internal/github/fake for the waiver; the real oasdiff, buf and k6 only in the floors workflow; never production

### Links

depends-on #496

### Checkpoints

1. `go test ./internal/verify -run 'TestContractFloor|TestAPIDefaults'` shows each spec kind, base-to-head comparison, the blocking skips, the generated smoke script, and the advisory date (criteria 1, 2, 3, 4, 5)
2. `go test . -run 'TestMergeGateAPITools'` (criterion 6)

### Assumptions

- a code-first API whose spec is not regenerated cannot show a break to this floor; Decision #480 records this gap
- the waiver for a breaking change is the accepted floor-waiver Decision that names contract
- `verify.new_floors_blocking_from` is 2026-11-14, shared by every floor Decision #480 added, and lands with #496

### Open questions

- deferred: a gRPC or GraphQL API with no spec in the repository shows no break to this floor (its spec is the only contract verify can read)

### Provenance

Request: "in proof floors should talk about code quality and user experience if there is a UI, or developer experience if it's an API or CLI. Goal: proof floors that, chosen by the story's layer footprint, cover: Developer experience, when it changes an API or a CLI (L02, or however LAYERS.md maps them). API DX: no breaking contract change (OpenAPI, GraphQL or proto, through tools like oasdiff or buf breaking) without a Decision, and a default k6 p95 budget. Ground rule 7 still holds: a floor is deterministic, and no model opinion can be a floor. It should list each floor, its default, the layers it applies to, the tool pinned by SHA-256, and what skipped means. Build only after it is accepted, one story per floor family, each with tests that a plausible wrong implementation would fail. Start the stories outside DAST now under #480's acceptance (code quality, UX, API and CLI developer experience)."
Term: "API" → a product's HTTP or RPC interface; "Developer experience, when it changes an API or a CLI"
Term: "CLI" → a product's command-line interface; "Developer experience, when it changes an API or a CLI"
Term: "DX" → developer experience; "API DX: no breaking contract change"
Term: "UI" → the product's user interface, layer L01; "user experience if there is a UI"
Term: "LAYERS" → LAYERS.md, the map of the nine layers and their owners; "however LAYERS.md maps them"
Term: "L02" → the Interaction / API boundary layer in LAYERS.md; "(L02, or however LAYERS.md maps them)"
Term: "L06" → the Contracts / integration layer in LAYERS.md; "however LAYERS.md maps them"
Term: "SHA-256" → the hash each downloaded tool is checked against; "the tool pinned by SHA-256"
Term: "DAST" → the ZAP scan of the running app, built in #496; "Start the stories outside DAST now"
Term: "OpenAPI" → the HTTP API description format; "OpenAPI, GraphQL or proto"
Term: "proto" → a protobuf schema; "OpenAPI, GraphQL or proto"
Term: "p95" → the 95th percentile of request duration; "a default k6 p95 budget"
1. stated (request): "no breaking contract change (OpenAPI, GraphQL or proto, through tools like oasdiff or buf breaking)"
2. stated (request): "OpenAPI, GraphQL or proto, through tools like oasdiff or buf breaking"
3. stated (request): "without a Decision" / "what skipped means"
4. stated (request): "a default k6 p95 budget"
5. stated (request): "Start the stories outside DAST now under #480's acceptance"
6. stated (request): "the tool pinned by SHA-256"
````
