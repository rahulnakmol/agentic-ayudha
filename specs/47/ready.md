# rahulnakmol/agentic-ayudha#47: WP4.7 Floors: screen reader

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `3072f3f9eedf357207575a7a39cce0c6c807d81a58a684e408220b9c624ed99d` |
| Completion Goal hash | `082fbb560696dc05f9a599fc77e4c43097a33a49600a1d3ac33304f466cebda0` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/47/ready.md` |

Completion Goal at Ready:

1. [sr-transcript] state: the WP4.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: fixture page with an unlabeled button fails against expected phrases
2. [sr-transcript] state: the WP4.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a PR touching only `internal/` paths skips the floor with `skipped: no-L01` in the pack

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP4.7 Floors: screen reader built as docs/BUILD-PLAN.md specifies, so that M4 (Proof) delivers: Every merge carries a CI-authoritative proof pack: pack format, ledger with LFS and media store, deterministic floors, merge gate and inspect, thin-slice verbs.

Plan text (docs/BUILD-PLAN.md, WP4.7): Guidepup VoiceOver (macOS runner) and NVDA (Windows runner); expected phrases come from the **Ready snapshot**, not from the agent at run time; transcript diff; runs only for stories whose footprint includes L01 (footprint computed from `LAYERS.md` globs over the PR's changed paths; the `slice`-confirmed footprint is n/a until WP5.4), on label or nightly to control runner cost.

### Completion Goal

1. [sr-transcript] state: the WP4.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: fixture page with an unlabeled button fails against expected phrases
2. [sr-transcript] state: the WP4.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a PR touching only `internal/` paths skips the floor with `skipped: no-L01` in the pack

### Measure

WP4.7 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP4.7)

### Threshold

all 2 criteria

### Scenario

Open the WP4.7 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
