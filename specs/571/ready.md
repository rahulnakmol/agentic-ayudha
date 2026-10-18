# rahulnakmol/agentic-ayudha#571: Opt-in Copilot route for the ui-review lane

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `bcd8c84bb7af1be02f5d997a727edb6d87f2ddcb9f3ce1eb0ef23771823b6d15` |
| Completion Goal hash | `87e5ce059bf2aa107526e2d1076adde787ac3285bfe79cf2555a2d5fe8a6fa4f` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/571/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the user config sets `[models.ui-review.copilot] model = "gemini-4-argon"` and verify reviews one screenshot (TestCopilotUIReviewRoute); expected: all 3 lane A calls run through Copilot CLI at effort high with the screenshot attached, no gateway call is made, and the actions log charges the AI credits Copilot reported (via copilot, cost_source reported)
2. [api-trace] state: the opt-in is set but copilot is not installed, or its reply does not parse (TestCopilotUIReviewRoute); expected: lane A falls back to gemini-3.8-flash (high) for that call and every later one, says so on stderr and in the label, and still returns findings
3. [api-trace] state: the opt-in is set in the repository's .aayudha/config.toml (TestRejects); expected: config loading fails with "never from the checkout" and no Copilot call is made
4. [api-trace] state: the recorded live gemini-4-argon call through Copilot CLI on the planted-defect screenshot (internal/models/testdata/copilot, 2026-10-18) replays through the route (TestCopilotCLI); expected: the prompt goes on stdin, only COPILOT_GITHUB_TOKEN of the caller's credentials reaches the process, the reply parses as a lane A reply, and the call is charged 0.042566 USD for 19,098 input tokens

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As a person with a Copilot plan that enables Gemini 4 Argon, I want to opt the ui-review lane into Copilot from my own config, so that screenshot review can run on Argon without a paid Google plan while Gemini 3.8 Flash (high) stays the default.

### Completion Goal

1. [api-trace] state: the user config sets `[models.ui-review.copilot] model = "gemini-4-argon"` and verify reviews one screenshot (TestCopilotUIReviewRoute); expected: all 3 lane A calls run through Copilot CLI at effort high with the screenshot attached, no gateway call is made, and the actions log charges the AI credits Copilot reported (via copilot, cost_source reported)
2. [api-trace] state: the opt-in is set but copilot is not installed, or its reply does not parse (TestCopilotUIReviewRoute); expected: lane A falls back to gemini-3.8-flash (high) for that call and every later one, says so on stderr and in the label, and still returns findings
3. [api-trace] state: the opt-in is set in the repository's .aayudha/config.toml (TestRejects); expected: config loading fails with "never from the checkout" and no Copilot call is made
4. [api-trace] state: the recorded live gemini-4-argon call through Copilot CLI on the planted-defect screenshot (internal/models/testdata/copilot, 2026-10-18) replays through the route (TestCopilotCLI); expected: the prompt goes on stdin, only COPILOT_GITHUB_TOKEN of the caller's credentials reaches the process, the reply parses as a lane A reply, and the call is charged 0.042566 USD for 19,098 input tokens

### Measure

Completion Goal criteria observed in the pull request's tests

### Threshold

all 4 criteria

### Scenario

A person adds the opt-in to their own config and runs verify or review on a story with screenshots; the tests above exercise that path with a fake copilot replaying the recorded live call.

### Environment

GitHub Actions on the pull request, with synthetic fixtures and the recorded Copilot CLI output only (no token)

### Assumptions

Copilot CLI's non-interactive mode, its usage file and its AI-credit rates stay as probed on 2026-10-18; the route stays a local opt-in with no Copilot credential in CI (founder's decision).
It follows Story #60 (review, polish, baseline), which built lane A on the gateway.

### Open questions

answered: does Argon become a default? No; Gemini 3.8 Flash (high) stays every host's default and the only model the ux evals gate.

### Checkpoints

1. With the opt-in in the user config, `aayu verify` on a story with screenshots shows lane A labelled "gemini-4-argon via Copilot" and the actions log charges the reported credits (criteria 1, 4)
2. With copilot missing, the same run prints the fallback line on stderr and reviews on gemini-3.8-flash (high) (criterion 2)
3. The same opt-in in the repository's .aayudha/config.toml makes config loading fail (criterion 3)

### Provenance

Request: "File a small Story for the opt-in Copilot route for the ui-review lane"
1. confirmed (models.route/opt-in): "build step 1, the Copilot route for the ui-review lane, but opt-in" / "Record the credits as the call's cost"
2. confirmed (models.route/fallback): "if copilot isn't installed or signed in, or the reply doesn't parse, lane A falls back to the normal flash-high path and says so; it never skips the review"
3. confirmed (models.route/trust): "Argon stays a local opt-in via [models.ui-review.copilot]"
4. confirmed (models.route/evidence): "a Completion Goal that #548's existing tests and the one recorded live call can prove"
````
