# rahulnakmol/agentic-ayudha#68: WP7.7 `evolve`

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `781366a6486bac093609936fddb389dbcb10b6fb69524f5ae32a0c098715b8b0` |
| Completion Goal hash | `049c320e3030de2241b05a284714f85207e4eaa380e66b910e1f7a7e7255757f` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/68/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the WP7.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: an `evolve` PR without a new eval case is rejected by CI

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP7.7 `evolve` built as docs/BUILD-PLAN.md specifies, so that M7 (Loops and explain) delivers: Scheduled loops, measure, brief, tell, audit and retro, evolve, release.

Plan text (docs/BUILD-PLAN.md, WP7.7): Quarterly: accepted retro Feedback → first a failing eval case that reproduces it → skill diff that passes → PR labelled `agent-authored`, human CODEOWNER review, never auto-merge; ships via `rolling`; `install --pin` rollback; per-skill correction rate tracked before/after.

### Completion Goal

1. [api-trace] state: the WP7.7 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: an `evolve` PR without a new eval case is rejected by CI

### Measure

WP7.7 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP7.7)

### Threshold

all 1 criteria

### Scenario

Open the WP7.7 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
