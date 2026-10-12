# rahulnakmol/agentic-ayudha#130: Plan documents store loads the synthetic Example Co seed

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `b32e4703a0373c59d075519104983f5633280fbe6dbc2e830c117e83dbc8f991` |
| Completion Goal hash | `ff4ed065495c8ba0030c3b1808f7d3b711f80aa45ddaa40c725098f7c28ee649` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/130/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the embedded seed `examples/benefits-qa/store/seed/plans.json` is loaded; expected: 16 passages of Example Co plan year 2026, each with a unique slug ID, a plan, a section, and text, and the seed is marked synthetic
2. [api-trace] state: a seed that is not marked synthetic, or has a duplicate or malformed ID, an empty field, or an email address outside example.com; expected: loading refuses it with an error that names the passage and the problem

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the benefits-qa example, I want the Example Co plan documents loaded from a checked, synthetic seed, so that every answer quotes known plan text and no capture can show personal data.

### Completion Goal

1. [api-trace] state: the embedded seed `examples/benefits-qa/store/seed/plans.json` is loaded; expected: 16 passages of Example Co plan year 2026, each with a unique slug ID, a plan, a section, and text, and the seed is marked synthetic
2. [api-trace] state: a seed that is not marked synthetic, or has a duplicate or malformed ID, an empty field, or an email address outside example.com; expected: loading refuses it with an error that names the passage and the problem

### Measure

Completion Goal criteria observed

### Threshold

all 2 criteria

### Scenario

Load the embedded seed, then load seeds with each planted defect.

### Environment

local checkout with the embedded synthetic Example Co seed; synthetic data only (V3), never production
````
