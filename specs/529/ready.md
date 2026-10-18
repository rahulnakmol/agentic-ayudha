# rahulnakmol/agentic-ayudha#529: slo measures a crash-free sessions SLO from Sentry release health over the whole SLO window

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `09e8aa28dec04619a5bd5ff41abe50748468ca12136cf460e6b243381d871040` |
| Completion Goal hash | `313d3ead1deaaf1f8829548b9cd7661c49e97f55887f1acb7562f944710bb46e` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/529/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: an `ops/slos.toml` with a `crash-free` SLO, and others that give it a threshold or a route, or name an unknown sli; expected: the first parses, each other is refused naming the field, and the refusal lists `crash-free` among the valid sli values
2. [api-trace] state: a recorded Sentry sessions answer grouped by session status over a 28-day window, with crashed sessions in the last hour and the last six hours; expected: good is every session but the crashed ones, total is every session, the value, the budget left, and the 1-hour and 6-hour burn rates match the counts, and the measure covers the whole window
3. [api-trace] state: the same measure, twice; expected: one GET to the organization's sessions endpoint scoped to the project, with the window's start and end, the sum of sessions grouped by session status, and no key in the URL, and the same SHA-256 over the answer both times, recorded with the source in the measure
4. [api-trace] state: `aayu slo --telemetry <export> --record` over definitions with an availability SLO and a crash-free SLO, with Sentry answered from the recording; expected: the status written to the ledger holds both measures, the crash-free one with its source and digest, and with no Sentry integration configured the crash-free SLO is reported as not measured and the status is not recorded
5. [api-trace] state: the release gate given a status whose crash-free SLO has spent its budget; expected: a Story is blocked naming that SLO, as for a spans SLO

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the Production Engineer, I want an SLO on crash-free sessions measured from the product's Sentry release health, so that `aayu slo` and the release gate can hold a mobile or desktop app to the reliability its users feel, not only to request spans (Decision #449, phase P1). Today `ops/slos.toml` accepts `sli = availability` or `latency`, both measured over request spans from an export (`internal/production/slo.go`). This Story adds `sli = "crash-free"`, whose good event is a session that did not crash. `aayu slo --record` measures it from Sentry's sessions API over the whole SLO window as aggregates only, and records the source and a SHA-256 of the answer in `slo/status.json`, which the release gate reads.

### Completion Goal

1. [api-trace] state: an `ops/slos.toml` with a `crash-free` SLO, and others that give it a threshold or a route, or name an unknown sli; expected: the first parses, each other is refused naming the field, and the refusal lists `crash-free` among the valid sli values
2. [api-trace] state: a recorded Sentry sessions answer grouped by session status over a 28-day window, with crashed sessions in the last hour and the last six hours; expected: good is every session but the crashed ones, total is every session, the value, the budget left, and the 1-hour and 6-hour burn rates match the counts, and the measure covers the whole window
3. [api-trace] state: the same measure, twice; expected: one GET to the organization's sessions endpoint scoped to the project, with the window's start and end, the sum of sessions grouped by session status, and no key in the URL, and the same SHA-256 over the answer both times, recorded with the source in the measure
4. [api-trace] state: `aayu slo --telemetry <export> --record` over definitions with an availability SLO and a crash-free SLO, with Sentry answered from the recording; expected: the status written to the ledger holds both measures, the crash-free one with its source and digest, and with no Sentry integration configured the crash-free SLO is reported as not measured and the status is not recorded
5. [api-trace] state: the release gate given a status whose crash-free SLO has spent its budget; expected: a Story is blocked naming that SLO, as for a spans SLO

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Parse SLO definitions with the new sli, measure a crash-free SLO from a recorded Sentry sessions answer, record the status through `aayu slo`, and run the release gate on it.

### Environment

local checkout and this repository's CI build job on a pull request; recorded answers shaped from Sentry's OpenAPI specification, no provider account

### Checkpoints

1. `go test ./internal/production -run 'TestParseSLOs|TestMeasureSentry'` shows the definitions and the measure with its request and digest (criteria 1, 2, 3)
2. `go test ./internal/cli -run 'TestSLOCrashFree|TestReleaseGate'` shows the recorded status and the gate (criteria 4, 5)

### Assumptions

- the recording is shaped from getsentry/sentry-api-schema at b1eac6b, operation "Retrieve Release Health Session Statistics" and its QuerySessions example, read 2026-10-17; not live-proven
- a crash-free session is any session whose status is not `crashed`, as Sentry's `crash_free_rate(session)` counts it
- an SLO window is up to 90 days, longer than investigate's 7-day query bound, so a provider measure is aggregates only, over exactly the SLO window, and never rows
- the Sentry project is the one `[integrations.sentry] project` names; an SLO's service is the journey's name, not a project selector
- Sentry's public API returns no uptime check results for one monitor (its detector response does not carry the identifier the results are keyed by), so an `uptime` sli is not part of this Story

### Provenance

Request: "Map every PostHog and Sentry capability that helps inspect, review, or improve a product to the aayu verb, seat, and layer that would use it"
Term: "native support" → Decision #449 option 1: a native read-only adapter for what feeds a check; "accepted, keys later, 3 yes, 4 yes"
Vague: "improve" → criteria 2, 5
1. stated (Production Engineer, consulted 2026-10-17): "please add them as distinct sli values (for example uptime and crash-free) with their own good-event definition"
2. stated (Production Engineer, consulted 2026-10-17): "Crash-free is sessions-based"
3. stated (Production Engineer, consulted 2026-10-17): "aggregates only, the window equal to the SLO window, and the source plus a digest in slo/status.json"
4. stated (Decision #449): "`slo` computes crash-free, availability, and latency SLIs from Sentry"
5. stated (Production Engineer, consulted 2026-10-17): "The release gate reads that file, so the gate tests need updating too"
````
