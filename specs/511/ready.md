# rahulnakmol/agentic-ayudha#511: investigate's logs and exceptions queries can be answered from a PostHog project, with the export's columns and a checkable digest

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `2634f3d519caf3b4a8dc3efbf9088a3e8059aaef2d1c86083c27f495e7b4100d` |
| Completion Goal hash | `861fbf9d68e425cb99f2c84ce91cbe410aee0582723ace5b36ac0992b07910a3` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/511/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: recorded PostHog answers for `exceptions where service=web` and `logs where service=web level=ERROR limit 1` over a one-hour window; expected: the exceptions result has the export's columns (version, type, message, top_frame, count, first, last) with version and top_frame empty and an email in a message redacted, and the logs result has the export's log columns with the service version and lower-case trace ID, truncated to one row of two
2. [api-trace] state: the same runs; expected: each first reads the key's scopes, then sends one POST to the project's query endpoint with the window as explicit dates and one row more than the limit, and returns that request and a SHA-256 over it and the answer; the exceptions provenance says first and last are each issue's lifetime first and last seen
3. [api-trace] state: `requests`, `exceptions by route`, `exceptions where version=2.4`, `logs where route=/x`, and `logs where level=loud`; expected: each is refused before any request, naming what PostHog can answer

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the Production Engineer investigating an incident in a product that sends its logs and exceptions to PostHog, I want investigate's logs and exceptions queries answered from the PostHog project directly, with the same columns, bounds, and redaction as an exported trace file and a digest that keeps each cited query checkable (Decision #449, phase P1). Today investigate reads only an exported OpenTelemetry directory, and PostHog's error tracking and logs are reachable only through a host's own PostHog tools. This Story adds a read-only PostHog source in `internal/production` that answers `logs` from a LogsQuery and `exceptions` from an ErrorTrackingQuery, after checking that the key reads only. Wiring it into `aayu investigate` comes with the P1 skill text.

### Completion Goal

1. [api-trace] state: recorded PostHog answers for `exceptions where service=web` and `logs where service=web level=ERROR limit 1` over a one-hour window; expected: the exceptions result has the export's columns (version, type, message, top_frame, count, first, last) with version and top_frame empty and an email in a message redacted, and the logs result has the export's log columns with the service version and lower-case trace ID, truncated to one row of two
2. [api-trace] state: the same runs; expected: each first reads the key's scopes, then sends one POST to the project's query endpoint with the window as explicit dates and one row more than the limit, and returns that request and a SHA-256 over it and the answer; the exceptions provenance says first and last are each issue's lifetime first and last seen
3. [api-trace] state: `requests`, `exceptions by route`, `exceptions where version=2.4`, `logs where route=/x`, and `logs where level=loud`; expected: each is refused before any request, naming what PostHog can answer

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Replay recorded PostHog answers through the real PostHog adapter into the PostHog source for logs and exceptions, compare the rows and the requests, and run the refused queries against a source that fails on any request.

### Environment

local checkout and this repository's CI build job on a pull request; recorded PostHog answers shaped from PostHog's OpenAPI specification, no PostHog account

### Checkpoints

1. `go test ./internal/production -run 'TestPostHogSource'` shows each table, the request, the digest, and the refusals (criteria 1, 2, 3)

### Assumptions

- the recordings are shaped from PostHog's OpenAPI 3.1 v1.0.0 (us.posthog.com/api/schema), read 2026-10-17, and the logs rows from PostHog's logs query runner at 8bb4013 because the specification types them as unknown; no PostHog account recorded them, so the source is not live-proven
- the query's service filters PostHog logs by service name; error tracking has no service or release breakdown, so exceptions rows leave version empty
- PostHog's query endpoint is the only POST the adapter may send (`integrations.ReadOnly`)
- wiring the source into `aayu investigate` changes the investigate skill, so it lands with the P1 skill text

### Provenance

Request: "Map every PostHog and Sentry capability that helps inspect, review, or improve a product to the aayu verb, seat, and layer that would use it"
Term: "native support" → Decision #449 option 1: a native read-only adapter for what feeds a cited number; "accepted, keys later, 3 yes, 4 yes"
Vague: "inspect" → criteria 1, 2
1. stated (Decision #449): "Error tracking … investigate · PE · L07" and "Logs (`LogsQuery`) … investigate · PE · L07"
2. stated (request): "Every call made through aayu, a host MCP, or a CLI is logged and cited like aayu's own queries (tool, window, row limit)"
3. stated (request): "Integrations read by default"
````
