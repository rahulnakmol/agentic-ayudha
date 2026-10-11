# rahulnakmol/agentic-ayudha#39: WP2.8 `status`

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `7ce829ef3aeb7f5f3273141c307993374976b89cfa59123d33407f2698069a71` |
| Completion Goal hash | `60739f9412be3591b6373b63569347cbf28a77b0057a89d32809a748b0905da9` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/39/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the WP2.8 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: asymmetric fixture tree with known rollups
2. [api-trace] state: the WP2.8 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: digest golden file

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP2.8 `status` built as docs/BUILD-PLAN.md specifies, so that M2 (Work items) delivers: A founder can init a repository, author issues through forms, make them Ready, see status, and triage intake (§4 'founder-usable early' after PR12).

Plan text (docs/BUILD-PLAN.md, WP2.8): Sub-issue walk with pagination; rollups per Goal/Initiative/Feature: counts by status, health, risk, **proof coverage** (stories whose pack maps every criterion and passed/waived all floors ÷ done stories; "n/a" until M4), streak per layer footprint (n/a until M5), blocked/over-budget, shipped-but-unmeasured, decisions near SLA (business hours from `[time]`) and **provisional decisions** (`decision:provisional`, with the hours until they must be ratified or reverted; n/a until WP5.6), cost. Outputs: table, `--json`, Goal comment (replace), `--digest` via `[notify]` (sender from WP1.6; the digest's proof and streak columns fill in at M4/M5/M7), MCP resource. `status --all` iterates `[fleet] repos` and prints one row per repo with its top Goal's rollup. Parent-close rule: Feature closes when all children `released` and a readout/Result exists; Initiative/Goal close on `measure` against baseline.

### Completion Goal

1. [api-trace] state: the WP2.8 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: asymmetric fixture tree with known rollups
2. [api-trace] state: the WP2.8 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: digest golden file

### Measure

WP2.8 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP2.8)

### Threshold

all 2 criteria

### Scenario

Open the WP2.8 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
