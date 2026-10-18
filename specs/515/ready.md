# rahulnakmol/agentic-ayudha#515: Host call log writes PostHog and Sentry tokens typed inline to disk

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `38a6d8e1cf4d0903c9001147af93956cd199d290048d6cfc0cd063be2952d8dc` |
| Completion Goal hash | `9195b3b7ac3e6f118473828cbc557489f98a0985dc139686a9c7457f5082089f` |
| Technical Owner | devops-devex-tinkerer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/515/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: log entries built for host calls that carry a token as an environment assignment, after `export`, in an `--auth-token=` flag, quoted in `POSTHOG_PERSONAL_API_KEY="…"`, in an `Authorization: Bearer` header, and as a bare `sntryu_` or `phx_` token; expected: no entry holds the token, and the variable or flag name stays with only its value replaced
2. [api-trace] state: the shared redaction every aayudha log uses (catalog payloads, provider error excerpts, eval excerpts); expected: it replaces PostHog `phx_` and `phs_` keys, Sentry `sntrys_` and `sntryu_` tokens, and bearer tokens, as it already replaced GitHub tokens

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

The host call log writes a PostHog or Sentry token to disk when an agent types it inline. The post-tool hook logs each PostHog and Sentry call a host ran to `.aayudha/state/calls/<session>.jsonl` (Decision #449, phase P0). A shell call is stored as its full command, and redaction only knew GitHub, OpenAI, Slack, and private-key shapes, so `SENTRY_AUTH_TOKEN=… sentry-cli …`, `--auth-token=…`, a quoted `POSTHOG_PERSONAL_API_KEY="phx_…"`, and an `Authorization: Bearer …` header all reached the file in clear text.

### Steps to reproduce

1. In a repository set up with `aayu install`, let the host run `SENTRY_AUTH_TOKEN=<token> sentry-cli issues list --limit 5 --period 1d`.
2. Read `.aayudha/state/calls/<session>.jsonl`.

### Expected behaviour

The entry's arguments keep the command's shape with every token value replaced by `[REDACTED]`: `SENTRY_AUTH_TOKEN=[REDACTED] sentry-cli issues list …`.

### Actual behaviour

The entry's `args.command` holds the token as typed.

### Completion Goal

1. [api-trace] state: log entries built for host calls that carry a token as an environment assignment, after `export`, in an `--auth-token=` flag, quoted in `POSTHOG_PERSONAL_API_KEY="…"`, in an `Authorization: Bearer` header, and as a bare `sntryu_` or `phx_` token; expected: no entry holds the token, and the variable or flag name stays with only its value replaced
2. [api-trace] state: the shared redaction every aayudha log uses (catalog payloads, provider error excerpts, eval excerpts); expected: it replaces PostHog `phx_` and `phs_` keys, Sentry `sntrys_` and `sntryu_` tokens, and bearer tokens, as it already replaced GitHub tokens

### Measure

Completion Goal criteria observed

### Threshold

all 2 criteria

### Scenario

Build log entries for host calls that carry tokens typed in each form, through the same code the post-tool hook runs, and search each entry for the token.

### Environment

local checkout and this repository's CI build job on a pull request; no provider account

### Technical Owner

devops-devex-tinkerer

### Checkpoints

1. `go test ./internal/integrations/hostcall -run TestNewEntry` shows every token form redacted with its name kept (criteria 1, 2)

### Assumptions

- Sentry's older user tokens are 64 hex characters with no prefix, so only their context (a `*TOKEN=` assignment, a token flag, an Authorization header) marks them; context redaction covers them
- redacting a value that only looks like a secret in context (`max_token=5`) costs a little detail in a log and is acceptable

### Provenance

Request: "The call log (.aayudha/state/calls) doesn't redact PostHog or Sentry tokens typed inline, such as "SENTRY_AUTH_TOKEN=… sentry-cli …". That writes a secret to disk: highest priority."
1. stated (request): "doesn't redact PostHog or Sentry tokens typed inline, such as "SENTRY_AUTH_TOKEN=… sentry-cli …""
2. stated (request): "That writes a secret to disk"
````
