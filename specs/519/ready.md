# rahulnakmol/agentic-ayudha#519: Host PostHog and Sentry recording, replay, and feedback reads pass without allow_pii

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `5f7ffc42459c4eb1868ccef42c66c3eee5ded276eeb67deb37e02b71a1345051` |
| Completion Goal hash | `3c47d2d230a8b227ab484e4db83d7b1527e871bc878c7e588d6dc2353d17836e` |
| Technical Owner | devops-devex-tinkerer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/519/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the pre-tool decision for PostHog `session-recording-get`, `query-session-recordings-list`, `session-recording-playlist-get`, and `vision-observations-list`, and Sentry `search_replays`, with allow_pii off and on; expected: refused with the allow_pii reason when off, allowed when on
2. [api-trace] state: the pre-tool decision for `sentry replay view` and `sentry feedback list` with allow_pii off and on; expected: refused with the allow_pii reason when off, allowed when on; `sentry issue view` still passes either way

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

The pre-tool hook lets an agent read session recordings and other personal data through a host's PostHog or Sentry MCP server or CLI while `allow_pii` is off (Decision #449: personal data is opt-in). The read allowlists put PostHog's recording tools (`session-recording-get`, `query-session-recordings-list`, the recording playlist tools, and the Replay Vision observation reads) and Sentry's `search_replays` outside the `[pii]` section. The new `sentry` CLI's `replay` and `feedback` reads (`sentry replay view`, `sentry feedback list`) passed with no opt-in check at all.

### Steps to reproduce

1. In a repository set up with `aayu install` and no `allow_pii`, let the host call PostHog's MCP tool `session-recording-get`.
2. Let it run `sentry replay view <id>`.

### Expected behaviour

Both are refused with the reason that they return personal data and the exact opt-in: `[integrations.<provider>] allow_pii = true` in `.aayudha/config.toml`.

### Actual behaviour

Both pass the hook.

### Completion Goal

1. [api-trace] state: the pre-tool decision for PostHog `session-recording-get`, `query-session-recordings-list`, `session-recording-playlist-get`, and `vision-observations-list`, and Sentry `search_replays`, with allow_pii off and on; expected: refused with the allow_pii reason when off, allowed when on
2. [api-trace] state: the pre-tool decision for `sentry replay view` and `sentry feedback list` with allow_pii off and on; expected: refused with the allow_pii reason when off, allowed when on; `sentry issue view` still passes either way

### Measure

Completion Goal criteria observed

### Threshold

all 2 criteria

### Scenario

Run the pre-tool decision on each recording, replay, and feedback read with and without the opt-in, and read the verdict and its reason.

### Environment

local checkout and this repository's CI build job on a pull request; tool definitions from PostHog commit 8bb4013 and getsentry/sentry-mcp commit 5044167, read 2026-10-17; no provider account

### Technical Owner

devops-devex-tinkerer

### Checkpoints

1. `go test ./internal/integrations/hostcall -run TestDecide` shows each personal-data read refused without the opt-in and allowed with it (criteria 1, 2)

### Assumptions

- every PostHog tool that needs the `session_recording:read` scope returns recordings or what was seen in them, so the rule is the scope, not a list of names
- Sentry's replay list names each replay's user, so it is personal data like the replay's details

### Provenance

Request: "hostcall: PostHog session-recording-get and the playlist tools are outside [pii], so allow_pii doesn't gate them. CLI personal-data reads, such as "sentry replay view", aren't gated by allow_pii."
Term: "pii" → personal data: person records, recordings and replays, user feedback, support tickets, raw LLM conversations; "personal data is opt-in"
Term: "MCP" → Model Context Protocol, the protocol a host uses to call a provider's tool server; "PostHog's or Sentry's own MCP server"
1. stated (request): "PostHog session-recording-get and the playlist tools are outside [pii], so allow_pii doesn't gate them"
2. stated (request): "CLI personal-data reads, such as "sentry replay view", aren't gated by allow_pii"
````
