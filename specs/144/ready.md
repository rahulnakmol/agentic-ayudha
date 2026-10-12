# rahulnakmol/agentic-ayudha#144: Release: benefits-qa example on the next rolling snapshot

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, release |
| Body hash | `c47e6b57e3fa7873bacb1804eb09a7b562b462844c5d66c25b90f616ee5b75d1` |
| Completion Goal hash | n/a (no Completion Goal on this type) |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/144/ready.md` |

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

Ships the benefits-qa example product (`examples/benefits-qa/`, WP8.1) on the next rolling snapshot: grounded answers through `POST /api/ask` (Feature #127), answer analytics as an Application Insights export with reproducible synthetic exports (Feature #128), and the accessible Ask page with its seeded run script (Feature #129). Serves Goal #125. All data is synthetic (V3).

### Version

the next rolling snapshot (rolling-<date>), promoted by the orchestrator with `aayu release`

### Links

includes #127
includes #128
includes #129
````
