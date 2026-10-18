# rahulnakmol/agentic-ayudha#608: inspect's rung B fails with 524 through the AI gateway on a large diff: Chat and Responses replies are not streamed

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `4ae5cffe8c38afa71b4e5a69edadd81616a002daf1c405be6cfc4b717d97600a` |
| Completion Goal hash | `6b008bcb4c327b7488efb3bb2ce1d98d2ef40f83930adb3be1d1619d1186d013` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/608/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a fake OpenAI-compatible endpoint behind a proxy that answers 524 when no byte arrives within its idle limit, with a reply that takes longer than that limit to finish; `aayu inspect` at rung B over Chat Completions and over Responses; expected: the request asks for a streamed reply, bytes arrive before the idle limit, and the decoded findings equal the non-streamed reply's
2. [api-trace] state: the same endpoint answering a plain JSON reply to a request that asked for streaming; expected: the reply still decodes
3. [api-trace] state: the endpoint answering 524 to every attempt; expected: inspect's error names the reviewer endpoint's timeout and its next step, and no disposition is posted

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

The merge gate's `aayudha/inspect` job fails with `aayu: model endpoint returned 524: error code: 524` (exit 5) on PR #603. It failed three times on head a160551 (run 39110530432, attempts 1 to 3). This repository's rung B reviews through the AI gateway with grok-4.7 at high effort (#592). That call uses the Chat Completions or Responses wire format, and `internal/models/client.go` streams only Anthropic Messages (`Request.Stream`). A long high-effort generation therefore sends no bytes, and the gateway's Cloudflare front answers 524 after 100 seconds of silence. The comment on `Request.Stream` already names this failure for Messages. Smaller PRs pass because their reply comes back in time. The PR gets no inspect disposition at its head, so under the merge policy it cannot merge, even though every required check is green.

### Steps to reproduce

1. Open a PR with a diff of about 600 changed lines (PR #603: 10 files, 576 lines) on this repository, where rung B goes through the AI gateway.
2. Let the merge gate run `aayudha inspect <pr> --ci`.

### Expected behaviour

1. A long rung-B generation over Chat Completions or Responses keeps bytes flowing (a streamed reply), so the gateway's proxy never answers 524, and inspect posts a disposition for the head.
2. When the endpoint still answers 524 or another 5xx, inspect says the reviewer endpoint timed out and what to do next, rather than only `aayu doctor`.

### Actual behaviour

1. Every attempt fails after about 3.5 minutes with `model endpoint returned 524` and exit 5. No inspect comment is posted for the head; the last comment still holds the previous head's `fix`.
2. The message's next step is `aayu doctor`, which cannot fix a proxy timeout.

### Completion Goal

1. [api-trace] state: a fake OpenAI-compatible endpoint behind a proxy that answers 524 when no byte arrives within its idle limit, with a reply that takes longer than that limit to finish; `aayu inspect` at rung B over Chat Completions and over Responses; expected: the request asks for a streamed reply, bytes arrive before the idle limit, and the decoded findings equal the non-streamed reply's
2. [api-trace] state: the same endpoint answering a plain JSON reply to a request that asked for streaming; expected: the reply still decodes
3. [api-trace] state: the endpoint answering 524 to every attempt; expected: inspect's error names the reviewer endpoint's timeout and its next step, and no disposition is posted

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Run the models client tests and the inspect rung-B tests in internal/cli against a fake endpoint with a proxy idle limit.

### Environment

aayu dev at 8f78cea (internal/models/client.go `Request.Stream`, StyleChat and StyleResponses), merge gate on ubuntu-latest, AI gateway behind Cloudflare.

### Checkpoints

1. Stream Chat Completions and Responses replies and decode their event streams (criteria 1, 2)
2. Name a reviewer timeout in inspect's error (criterion 3)

### Links

- caused-by #592

### Assumptions

The gateway passes `stream: true` through to grok-4.7 for both wire formats; if it does not, the fix needs the gateway's own keep-alive instead.

### Provenance

Request: "merging agent PRs into dev must be frictionless. Every PR to dev, Story or not, squash-merges as soon as every required check is green and inspect's disposition is merge"
1. stated (reliability/inspect disposition): "squash-merges as soon as every required check is green and inspect's disposition is merge"
2. stated (reliability/inspect disposition): "inspect's disposition is merge"
3. stated (usability/frictionless): "merging agent PRs into dev must be frictionless"
Vague: "frictionless" → criteria 1, 3
````
