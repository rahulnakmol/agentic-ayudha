# rahulnakmol/agentic-ayudha#60: WP6.6 `review`, `polish`, `baseline`

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `6baa33740e96bae931dfed423b7ae3254199299d253e2dadcce70ccec450d3fe` |
| Completion Goal hash | `de1591ea1d6087ef406c65e0a287e4bccb8acf60708f5163806c540adb22a436` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/60/ready.md` |

Completion Goal at Ready:

1. [screenshot] state: the WP6.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: three planted defects (missing focus ring, 3.8:1 contrast on body text, 32 px tap target) are each caught by lane A in ≥ 2 of 3 runs and by lane B in 1 of 1
2. [screenshot] state: the WP6.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a clean fixture yields `ship` from both lanes
3. [screenshot] state: the WP6.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `polish` on the planted fixture removes all three on re-run

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP6.6 `review`, `polish`, `baseline` built as docs/BUILD-PLAN.md specifies, so that M6 (UX) delivers: Figma and Mobbin references, research to prototype, review and polish, experiments and readouts.

Plan text (docs/BUILD-PLAN.md, WP6.6): Lane A (Gemini 3.1 Pro / Fable 5.1 medium, `--deep` high) with Nielsen 10×0–4 and personas incl. screen-reader; lane B (impeccable `detect --json`, Apache-2.0, shelled out; axe); isolated; dispositions `recapture|rebuild|fix|ship`; `polish` consumes critique; `baseline` per SHA. Lane A runs k = 3 and a finding counts only when ≥ 2 of 3 runs report it at severity ≥ 2 (reduces judge noise); lane B is deterministic.

### Completion Goal

1. [screenshot] state: the WP6.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: three planted defects (missing focus ring, 3.8:1 contrast on body text, 32 px tap target) are each caught by lane A in ≥ 2 of 3 runs and by lane B in 1 of 1
2. [screenshot] state: the WP6.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: a clean fixture yields `ship` from both lanes
3. [screenshot] state: the WP6.6 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `polish` on the planted fixture removes all three on re-run

### Measure

WP6.6 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP6.6)

### Threshold

all 3 criteria

### Scenario

Open the WP6.6 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
