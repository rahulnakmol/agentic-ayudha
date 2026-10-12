# rahulnakmol/agentic-ayudha#128: Answer analytics (benefits-qa)

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, feature |
| Body hash | `71a5ce259956c5416340c1a998e1f89d0b9a67b14bc65dd98c86866f4125960d` |
| Completion Goal hash | n/a (no Completion Goal on this type) |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/128/ready.md` |

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

Answer analytics: the product's events (visits, questions, grounded answers, referrals, refusals) as an Application Insights export that `aayu measure` reads, and a seeded simulation of synthetic employees that produces the exports the Goal #125 baseline and Result are measured on. Serves Initiative #126.

### Problem

The Goal's metric (activation) needs product events in a format `aayu measure` reads, and V3 forbids measuring real people: the baseline and the outcome must come from reproducible synthetic data.

### Scope

In scope: an Application Insights (Log Analytics JSON lines) recorder for the workflow's events, a deterministic simulation of a synthetic population through the HR-ticket channel and the product channel, and the two committed exports with a test that regenerates them byte for byte. Out of scope: real telemetry pipelines, dashboards, and any personal data.

### Expected paths

examples/benefits-qa/analytics/**

### Links

depends-on #127
````
