# rahulnakmol/agentic-ayudha#552: runbook checks each runbook's alerts against the configured Sentry and PostHog projects

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `1c66dfbb2ff62d57262ff02a0cb770164ba20feeb917cb10566be235e5749a03` |
| Completion Goal hash | `061dd49cb516149441faf75bedd04e73ba18360a8eea632eca2dbb9afa4ced7e` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/552/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: a checkout with one runbook whose Symptom names an alert Sentry has, an alert PostHog has, and an alert neither has, with both providers configured and answered from recordings; expected: `aayu runbook` exits 4 with one finding naming the missing alert, reports the other two as found in their providers, and the grounding names each provider's requests and SHA-256
2. [api-trace] state: the same checkout with no provider configured; expected: `aayu runbook` sends no request, reports every alert as not checked, and raises no alert finding
3. [api-trace] state: a Sentry token that Sentry reports as able to write, and a PostHog key PostHog rejects; expected: no alert list is read with either, each provider's alerts are not checked, the grounding names the reason, and no alert finding is raised

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the Production Engineer, I want `aayu runbook` to check each runbook's alerts against the product's configured Sentry and PostHog projects, so that a runbook that answers a renamed or deleted alert is caught before the alert fires (Decision #449, phase P1). The lookup itself merged in #540 (`production.RunbookAlerts`); nothing calls it yet. This Story wires it into `aayu runbook`: with Sentry or PostHog configured, it checks the token reads only, lists the project's alerts once, reports each alert a runbook's Symptom names as found (with the provider) or missing, and a missing alert is a finding. With no provider configured, or when a provider cannot be read, every alert is reported as not checked, never as a finding, and the grounding says why.

### Completion Goal

1. [api-trace] state: a checkout with one runbook whose Symptom names an alert Sentry has, an alert PostHog has, and an alert neither has, with both providers configured and answered from recordings; expected: `aayu runbook` exits 4 with one finding naming the missing alert, reports the other two as found in their providers, and the grounding names each provider's requests and SHA-256
2. [api-trace] state: the same checkout with no provider configured; expected: `aayu runbook` sends no request, reports every alert as not checked, and raises no alert finding
3. [api-trace] state: a Sentry token that Sentry reports as able to write, and a PostHog key PostHog rejects; expected: no alert list is read with either, each provider's alerts are not checked, the grounding names the reason, and no alert finding is raised

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Run `aayu runbook` on a test checkout through the real CLI, with Sentry and PostHog answered from recordings, configured, unconfigured, and with a refused token, and read the exit code, the findings, and the requests.

### Environment

local checkout and this repository's CI build job on a pull request; recorded answers shaped from Sentry's and PostHog's OpenAPI specifications, no provider account

### Checkpoints

1. `go test ./internal/cli -run 'TestRunbookAlertsLive'` shows the findings, the statuses, and the requests (criteria 1, 2, 3)

### Assumptions

- the recordings are shaped from getsentry/sentry-api-schema at b1eac6b and PostHog's OpenAPI 3.1 v1.0.0, read 2026-10-17; not live-proven
- the eval groundings in evals/cases are recorded files, so changing `aayu runbook`'s output does not change what the production eval group scores; the runbook skill text changes in a later Story
- a provider that is configured but cannot be read leaves its alerts not checked, like an unconfigured one, so a network failure never blocks a runbook

### Provenance

Request: "Map every PostHog and Sentry capability that helps inspect, review, or improve a product to the aayu verb, seat, and layer that would use it"
Term: "native support" → Decision #449 option 1: a native read-only adapter for what feeds a check; "accepted, keys later, 3 yes, 4 yes"
Vague: "review" → criteria 1, 2
1. stated (Decision #449): "Alerts … runbook (the alert the entry names exists and matches) · PE"
2. stated (Production Engineer, consulted 2026-10-17): "When no provider is configured, the result should be \"not checked\", never a finding"
3. stated (request): "Keys with the least scope, checked by doctor."
````
