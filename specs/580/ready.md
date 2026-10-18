# rahulnakmol/agentic-ayudha#580: investigate runs its bounded queries against the configured Sentry or PostHog project with --source

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `8346990e84daaa3ba969785501ced3c3efdc8bab287060b4b869bddcffc66283` |
| Completion Goal hash | `b89e4819534feff97eec5c7877f54b1ee7a04efbb9a3a189a4617de74749e282` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/580/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: an incident and `aayu investigate <incident> --source sentry --service web` with Sentry answered from recordings; expected: the grounding holds the requests, errors, exceptions, deploys, and logs queries as `query q1` to `query q5`, each with the Sentry requests it sent and a SHA-256, the deployed revision from the newest release deploy, and the implicated code from the exceptions' top frames; the query log file holds the same entries
2. [api-trace] state: `aayu investigate <incident> --source posthog --service web` with PostHog answered from recordings; expected: only the exceptions and logs queries run, each logged with its request and SHA-256, and a `--query` of a kind PostHog does not answer exits 2 naming the kinds it answers
3. [api-trace] state: `--source` with `--telemetry`, `--source` without `--service`, an unknown source, an unconfigured source, and a Sentry token that can write; expected: each exits with a usage or blocked error naming the fix, and no provider data request is sent

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the Production Engineer investigating an incident in a product that reports to Sentry or PostHog, I want `aayu investigate` to run its bounded queries against the configured project directly, so that I can investigate from live data without first exporting telemetry (Decision #449, phase P1). The sources merged in #526 (Sentry) and #536 (PostHog), but nothing calls them: `aayu investigate` reads only an export directory (`--telemetry`). This Story adds `--source sentry|posthog` as the alternative to `--telemetry`: aayu checks the key reads only, runs the default queries each source answers, logs each as a citable `qN` with the source's requests and a SHA-256 over the answers, takes the deployed revision from Sentry's release deploys when `--revision` is not given, and maps the exceptions' top frames to code at that revision. CI is refused as before.

### Completion Goal

1. [api-trace] state: an incident and `aayu investigate <incident> --source sentry --service web` with Sentry answered from recordings; expected: the grounding holds the requests, errors, exceptions, deploys, and logs queries as `query q1` to `query q5`, each with the Sentry requests it sent and a SHA-256, the deployed revision from the newest release deploy, and the implicated code from the exceptions' top frames; the query log file holds the same entries
2. [api-trace] state: `aayu investigate <incident> --source posthog --service web` with PostHog answered from recordings; expected: only the exceptions and logs queries run, each logged with its request and SHA-256, and a `--query` of a kind PostHog does not answer exits 2 naming the kinds it answers
3. [api-trace] state: `--source` with `--telemetry`, `--source` without `--service`, an unknown source, an unconfigured source, and a Sentry token that can write; expected: each exits with a usage or blocked error naming the fix, and no provider data request is sent

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Run `aayu investigate` through the real CLI with Sentry and PostHog answered from recordings, read the grounding, the query log, and the exit codes, and check the refusals send no data request.

### Environment

local checkout and this repository's CI build job on a pull request; recorded answers shaped from Sentry's and PostHog's OpenAPI specifications, no provider account

### Checkpoints

1. `go test ./internal/cli -run 'TestInvestigateLive'` shows the grounding, the query log, and the refusals (criteria 1, 2, 3)

### Assumptions

- the recordings are shaped from getsentry/sentry-api-schema at b1eac6b and PostHog's OpenAPI 3.1 v1.0.0, read 2026-10-17; not live-proven
- a live source has no export to name the service, so `--service` is required; the window defaults to an hour before the incident started, or the last 24 hours, up to now
- Sentry does not answer `timeline`, and PostHog answers only `logs` and `exceptions`, so each source runs the default queries it answers
- earlier logged queries stay citable, but a live source does not rerun them, because live data moves and each rerun spends quota
- the flag changes the generated frontmatter of the investigate skill; the skill's text changes in a later Story

### Provenance

Request: "Map every PostHog and Sentry capability that helps inspect, review, or improve a product to the aayu verb, seat, and layer that would use it"
Term: "native support" → Decision #449 option 1: a native read-only adapter for what feeds a check; "accepted, keys later, 3 yes, 4 yes"
Vague: "inspect" → criteria 1, 2
1. stated (Decision #449): "Every call made through aayu, a host MCP, or a CLI is logged and cited like aayu's own queries (tool, window, row limit)"
2. stated (Production Engineer, consulted 2026-10-17): "For live sources, record the source URL plus a sha256 of the response body, so a cited qN stays checkable"
3. stated (request): "Keys with the least scope, checked by doctor."
````
