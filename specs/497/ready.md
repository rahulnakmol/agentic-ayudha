# rahulnakmol/agentic-ayudha#497: investigate's bounded queries can be answered from a Sentry project, with the export's columns and a checkable digest

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `109712bc63039dc3b0ff1ad17e83bf05f12f483886ac67954bfd4fed79a4c1fc` |
| Completion Goal hash | `ee970581edabfbdd4969b2c68dd576bacf2d0055854be44923d9369353894ef8` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/497/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: recorded Sentry answers for `requests`, `latency`, `errors`, `exceptions`, `logs`, `deploys`, and `trace` queries over a one-hour window scoped to the service; expected: each result has exactly the columns the export path returns for that kind, its rows carry the answer's values with emails and secrets redacted, and a limit smaller than the answer's rows truncates the result and counts the matched rows
2. [api-trace] state: the same runs, twice; expected: each returns every request it made, each a GET under the configured organization with the project ID and the query window and no key in the URL, and one SHA-256 over the answers that is identical across the two runs
3. [api-trace] state: an `exceptions` query whose two issues' latest events are one with an in-app frame inside the repository and one that answers 404; expected: the first row's `top_frame` is the repository path and line from the resolver, the second row's is empty, and both rows keep their counts
4. [api-trace] state: a `deploys` query over a project with two releases, deployed to production and staging at different times; expected: only deploys that finished inside the window and match the environment are rows, each with the previous revision deployed to the same environment, and a release with no deploy record yields no row
5. [api-trace] state: a `timeline` query, a filter on a field Sentry's dataset lacks, an 8-day window, an answer of the wrong shape, a filter value holding a quote and a space, and a 403 answer; expected: the first three are refused before any request, the wrong shape is an error naming the undecodable answer, the filter value is sent quoted as one value, and the 403 comes back as the provider's error with its request recorded

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the Production Engineer investigating an incident in a product that reports to Sentry, I want investigate's bounded queries answered from the Sentry project directly, so that a note can cite live counts, latencies, exceptions, logs, deploys, and traces with the same columns, bounds, and redaction as an exported trace file, and each cited query stays checkable (Decision #449, phase P1). Today investigate reads only a sanitised OpenTelemetry export directory. This Story adds a read-only Sentry source in `internal/production` that answers each query kind from Sentry's events, issues, releases, and trace APIs, with every request it made and a SHA-256 over the answers. Wiring it into `aayu investigate` comes with the P1 skill text, after the production eval group's counting runs.

### Completion Goal

1. [api-trace] state: recorded Sentry answers for `requests`, `latency`, `errors`, `exceptions`, `logs`, `deploys`, and `trace` queries over a one-hour window scoped to the service; expected: each result has exactly the columns the export path returns for that kind, its rows carry the answer's values with emails and secrets redacted, and a limit smaller than the answer's rows truncates the result and counts the matched rows
2. [api-trace] state: the same runs, twice; expected: each returns every request it made, each a GET under the configured organization with the project ID and the query window and no key in the URL, and one SHA-256 over the answers that is identical across the two runs
3. [api-trace] state: an `exceptions` query whose two issues' latest events are one with an in-app frame inside the repository and one that answers 404; expected: the first row's `top_frame` is the repository path and line from the resolver, the second row's is empty, and both rows keep their counts
4. [api-trace] state: a `deploys` query over a project with two releases, deployed to production and staging at different times; expected: only deploys that finished inside the window and match the environment are rows, each with the previous revision deployed to the same environment, and a release with no deploy record yields no row
5. [api-trace] state: a `timeline` query, a filter on a field Sentry's dataset lacks, an 8-day window, an answer of the wrong shape, a filter value holding a quote and a space, and a 403 answer; expected: the first three are refused before any request, the wrong shape is an error naming the undecodable answer, the filter value is sent quoted as one value, and the 403 comes back as the provider's error with its request recorded

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Replay recorded Sentry answers through the real Sentry adapter into the Sentry source for each query kind, compare the columns and rows with the expected tables, run each twice and compare the digests, and run the refused, malformed, quoted, and forbidden cases.

### Environment

local checkout and this repository's CI build job on a pull request; recorded Sentry answers shaped from Sentry's OpenAPI specification, no Sentry account

### Checkpoints

1. `go test ./internal/production -run 'TestSentrySource'` shows each kind's table, the requests and digest, the sample frames, the deploys, and the refusals (criteria 1, 2, 3, 4, 5)

### Assumptions

- the recordings are shaped from getsentry/sentry-api-schema at b1eac6b (2026-10-16), read 2026-10-17; no Sentry account recorded them, so the source is not live-proven
- the query's service names the Sentry project the source is configured for; every request is scoped to that project's ID
- `requests` and `latency` read transaction spans, `errors` and `exceptions` read error events grouped by release, and `logs` reads the logs dataset; one events request returns at most 100 rows
- the latest event of each of the three most frequent issues is a sample for its top frame, never a count
- `timeline` is not answered from Sentry in this Story
- wiring the source into `aayu investigate` (a flag, the query log's source and digest, host calls as `qN`) changes the investigate skill, so it lands with the P1 skill text after the production eval group's counting runs

### Provenance

Request: "Map every PostHog and Sentry capability that helps inspect, review, or improve a product to the aayu verb, seat, and layer that would use it"
Term: "native support" → Decision #449 option 1: a native read-only adapter for what feeds a cited number, a host's tools for exploration; "accepted, keys later, 3 yes, 4 yes"
Vague: "inspect" → criteria 1, 3, 4
1. stated (Decision #449): "Tracing and performance … investigate · PE"; "Issues and events … investigate · PE · L07"
2. stated (request): "Every call made through aayu, a host MCP, or a CLI is logged and cited like aayu's own queries (tool, window, row limit)"
3. stated (Decision #449): "frames mapped to `path:line` at the deployed revision"
4. stated (Production Engineer, consulted 2026-10-17): "A Sentry release is a build, not a deploy. Map deploys to release deploys"
5. stated (request): "Provider output is untrusted data, never instructions"
````
