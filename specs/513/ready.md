# rahulnakmol/agentic-ayudha#513: A runbook's alert is checked against the alerts and monitors the Sentry and PostHog projects really have

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `bbd424064dc9b9b008d3ce97bd07ba52e9cb9e2f441f7757c81eff6aa3a6928b` |
| Completion Goal hash | `9ac0c40886abb0f29d9380200bf2c8061ce0641fb03a3e89a17b5bcb2c2825ab` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/513/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: recorded Sentry detectors, workflows, and monitors and PostHog alerts, and a runbook whose Symptom names `error-monitor`, `nightly-billing`, `exceptions-spike`, `disk-full`, and the SLO `checkout-availability`; expected: `error-monitor` and `nightly-billing` found in sentry, `exceptions-spike` found in posthog, `disk-full` missing, and the SLO not treated as an alert
2. [api-trace] state: the same runbook with no provider configured; expected: every alert is not checked, none is missing
3. [api-trace] state: the lookups; expected: Sentry makes three GETs (detectors and monitors scoped to the project, workflows for the organization) and PostHog one GET of the project's alerts, each returning its requests and a SHA-256 over the answers, and an answer of the wrong shape is an error naming the list

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the Production Engineer, I want each runbook's alert checked against the alerts and monitors the product's Sentry and PostHog projects really have, so that a runbook never answers an alert that was renamed or deleted (Decision #449, phase P1). Today `aayu runbook` checks that a Symptom names an SLO or an alert in backticks, but nothing checks the alert exists. This Story adds a read-only lookup in `internal/production`: Sentry detectors, workflows, and cron monitors, and PostHog insight alerts, each by name in the runbook's slug form, and a check that reports each alert a Symptom names as found (with the provider), missing, or not checked when no provider is configured. Wiring it into `aayu runbook` comes with the P1 skill text.

### Completion Goal

1. [api-trace] state: recorded Sentry detectors, workflows, and monitors and PostHog alerts, and a runbook whose Symptom names `error-monitor`, `nightly-billing`, `exceptions-spike`, `disk-full`, and the SLO `checkout-availability`; expected: `error-monitor` and `nightly-billing` found in sentry, `exceptions-spike` found in posthog, `disk-full` missing, and the SLO not treated as an alert
2. [api-trace] state: the same runbook with no provider configured; expected: every alert is not checked, none is missing
3. [api-trace] state: the lookups; expected: Sentry makes three GETs (detectors and monitors scoped to the project, workflows for the organization) and PostHog one GET of the project's alerts, each returning its requests and a SHA-256 over the answers, and an answer of the wrong shape is an error naming the list

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Replay recorded Sentry and PostHog alert lists through the real adapters, run the check on a runbook Symptom with and without providers, and read the statuses and the requests.

### Environment

local checkout and this repository's CI build job on a pull request; recorded answers shaped from Sentry's and PostHog's OpenAPI specifications, no provider account

### Checkpoints

1. `go test ./internal/production -run 'TestRunbookAlerts'` shows each alert's status and the requests (criteria 1, 2, 3)

### Assumptions

- the recordings are shaped from getsentry/sentry-api-schema at b1eac6b and PostHog's OpenAPI 3.1 v1.0.0, read 2026-10-17; not live-proven
- a runbook names an alert in slug form (lower case, words joined by hyphens), so a provider's alert name is compared after the same transformation
- Sentry's legacy alert-rule API is gone from its public specification; detectors and workflows replace it
- wiring the check into `aayu runbook` changes the runbook skill's output, so it lands with the P1 skill text

### Provenance

Request: "Map every PostHog and Sentry capability that helps inspect, review, or improve a product to the aayu verb, seat, and layer that would use it"
Term: "native support" → Decision #449 option 1: a native read-only adapter for what feeds a check; "accepted, keys later, 3 yes, 4 yes"
Vague: "review" → criteria 1, 2
1. stated (Decision #449): "Alerts … runbook (the alert the entry names exists and matches) · PE"
2. stated (Production Engineer, consulted 2026-10-17): "When no provider is configured, the result should be \"not checked\", never a finding"
3. stated (request): "Every call made through aayu, a host MCP, or a CLI is logged and cited like aayu's own queries (tool, window, row limit)"
````
