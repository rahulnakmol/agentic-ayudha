# rahulnakmol/agentic-ayudha#59: WP6.5 `research`, `wireframe`, `mockup`, `prototype`

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `aebe12f217a7ee5eb0dfddaf47f517bc1618faaf1a7b205f5464e08e6de23769` |
| Completion Goal hash | `e82613a3b19b5e968d9a1d7414b372307e0d5963304f64fb9094fbeec6bcc944` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/59/ready.md` |

Completion Goal at Ready:

1. [screenshot] state: the WP6.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: eval chain on a fixture feature produces all four with pinned references
2. [screenshot] state: the WP6.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a prototype fixture at 1.5 % diff passes and one at 2.5 % fails
3. [screenshot] state: the WP6.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a fixture using a non-token colour fails regardless of pixel diff

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP6.5 `research`, `wireframe`, `mockup`, `prototype` built as docs/BUILD-PLAN.md specifies, so that M6 (UX) delivers: Figma and Mobbin references, research to prototype, review and polish, experiments and readouts.

Plan text (docs/BUILD-PLAN.md, WP6.5): As settled; artifacts under `ledger/design/<feature>/`; prototype behind OpenFeature flag with Code Connect mapping; screenshot diff vs Figma with declared tolerance (`[review] figma_diff_max`, default 2 % of pixels after anti-aliasing mask; typography and colour token mismatches are counted separately and must be zero).

### Completion Goal

1. [screenshot] state: the WP6.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: eval chain on a fixture feature produces all four with pinned references
2. [screenshot] state: the WP6.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a prototype fixture at 1.5 % diff passes and one at 2.5 % fails
3. [screenshot] state: the WP6.5 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a fixture using a non-token colour fails regardless of pixel diff

### Measure

WP6.5 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP6.5)

### Threshold

all 3 criteria

### Scenario

Open the WP6.5 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
