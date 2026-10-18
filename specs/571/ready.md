# rahulnakmol/agentic-ayudha#571: UI review on Gemini 4 Argon through Copilot by default

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `b3afb3093aaa24a0110ee9ae1b98ad9f231228d6ec7f4ebf2dd036208a630d60` |
| Completion Goal hash | `2b7faa86e803da175c96a879c6f595a1deaa857725f1f63f06a2de2b084b2a21` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/571/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: copilot is signed in and the user config has no [models.ui-review.copilot] row, and verify reviews one screenshot (TestCopilotUIReviewRoute); expected: all 3 lane A calls run on gemini-4-argon through Copilot CLI at effort high with the screenshot attached, no gateway call is made, and the actions log charges the AI credits Copilot reported (via copilot, cost_source reported)
2. [api-trace] state: copilot is not signed in (TestVerifyScreenshotReview), or it fails or its reply does not parse (TestCopilotUIReviewRoute); expected: lane A runs on gemini-3.8-flash (high) through the gateway for that call and every later one, says why on stderr and in the label, and still returns findings
3. [api-trace] state: the user config sets [models.ui-review.copilot] model = "" (TestCopilotUIReviewRoute), or a repository's .aayudha/config.toml sets that row (TestRejects); expected: with the empty model lane A runs on gemini-3.8-flash without trying Copilot; the repository row fails config loading with "never from the checkout"
4. [api-trace] state: the recorded live gemini-4-argon call through Copilot CLI on the planted-defect screenshot (internal/models/testdata/copilot, 2026-10-18) replays through the route (TestCopilotCLI); expected: the prompt goes on stdin, only COPILOT_GITHUB_TOKEN of the caller's credentials reaches the process, the reply parses as a lane A reply, and the call is charged 0.042566 USD for 19,098 input tokens

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As a person who runs aayu with GitHub Copilot signed in, I want UI review to run on Gemini 4 Argon through Copilot by default and fall back to Gemini 3.8 Flash (high) otherwise, so that screenshot review uses the stronger model without a paid Google plan and always runs.

### Completion Goal

1. [api-trace] state: copilot is signed in and the user config has no [models.ui-review.copilot] row, and verify reviews one screenshot (TestCopilotUIReviewRoute); expected: all 3 lane A calls run on gemini-4-argon through Copilot CLI at effort high with the screenshot attached, no gateway call is made, and the actions log charges the AI credits Copilot reported (via copilot, cost_source reported)
2. [api-trace] state: copilot is not signed in (TestVerifyScreenshotReview), or it fails or its reply does not parse (TestCopilotUIReviewRoute); expected: lane A runs on gemini-3.8-flash (high) through the gateway for that call and every later one, says why on stderr and in the label, and still returns findings
3. [api-trace] state: the user config sets [models.ui-review.copilot] model = "" (TestCopilotUIReviewRoute), or a repository's .aayudha/config.toml sets that row (TestRejects); expected: with the empty model lane A runs on gemini-3.8-flash without trying Copilot; the repository row fails config loading with "never from the checkout"
4. [api-trace] state: the recorded live gemini-4-argon call through Copilot CLI on the planted-defect screenshot (internal/models/testdata/copilot, 2026-10-18) replays through the route (TestCopilotCLI); expected: the prompt goes on stdin, only COPILOT_GITHUB_TOKEN of the caller's credentials reaches the process, the reply parses as a lane A reply, and the call is charged 0.042566 USD for 19,098 input tokens

### Measure

Completion Goal criteria observed in the pull request's tests

### Threshold

all 4 criteria

### Scenario

A person with Copilot signed in runs verify or review on a story with screenshots; the tests above exercise that path with a fake copilot replaying the recorded live call, and the fallback and off paths.

### Environment

GitHub Actions on the pull request, with synthetic fixtures and the recorded Copilot CLI output only (no token)

### Checkpoints

1. With copilot signed in, `aayu verify` on a story with screenshots labels lane A "gemini-4-argon via Copilot" and the actions log charges the reported credits (criteria 1, 4)
2. Without copilot, the same run prints why on stderr and reviews on gemini-3.8-flash (high) (criterion 2)
3. With model = "" in the user config, the same run uses gemini-3.8-flash without trying Copilot (criterion 3)

### Assumptions

Copilot CLI's non-interactive mode, its usage file and its AI-credit rates stay as probed on 2026-10-18. CI has no Copilot credential (founder's decision), so CI and the ux evals always take the fallback and measure gemini-3.8-flash only.
It follows Story #60 (review, polish, baseline), which built lane A on the gateway.

### Open questions

answered: is Argon the default? Yes, wherever Copilot is installed and signed in (founder's decision, 2026-10-18); Gemini 3.8 Flash (high) otherwise, and in CI.

### Provenance

Request: "File a small Story for the opt-in Copilot route for the ui-review lane"
1. confirmed (models.route/default): "UI review uses Gemini 4 Argon through Copilot by default wherever it is available" / "Keep reviewer ≠ builder, the budget stop, and the credits accounting"
2. confirmed (models.route/fallback): "otherwise, or on any failure or schema miss, use flash-high as #548 already does, saying so on stderr and in the lane label"
3. confirmed (models.route/off): "Keep a way to turn it off ([models.ui-review.copilot] model = \"\" or similar) and document it"
4. confirmed (models.route/evidence): "a Completion Goal that #548's existing tests and the one recorded live call can prove"
````
