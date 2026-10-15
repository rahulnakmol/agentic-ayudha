# rahulnakmol/agentic-ayudha#177: The budget breaker counts one process's spend: Breaker.Seed is never called

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `890228a5256a6ebc7e83244b0d2af811602edb8696a77936afb4ff189bace195` |
| Completion Goal hash | `794fe0aa98dfb27071f6414f605c97295ebfffdd9e723305fafe62444d9bf931` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/177/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: two `aayu` processes in one checkout on the same day, each making model calls that cost 0.60 USD, with `[budget] per_day_usd = 1`; expected: the second process's call trips the breaker: it exits 6, the pause file is written, and one page names the day's spend
2. [api-trace] state: two processes in one checkout spending 0.60 USD each on the same story, with `[budget] per_story_usd = 1` and a high per-day limit; expected: the second process trips the breaker for that story with exit 6
3. [api-trace] state: an actions log holding 0.90 USD of model calls from yesterday and a new process today with `[budget] per_day_usd = 1`; expected: a 0.60 USD call today does not trip the breaker

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

The `[budget]` circuit breaker (`models.Breaker` in `internal/models/gateway.go`) keeps its per-day and per-story spend in memory. `Breaker.Seed`, which loads earlier spend from actions records, is defined but never called: `Env.Gateway` (`internal/cli/triage.go`) builds a fresh `Breaker` for every process. Each `aayu` process therefore starts at zero, so `[budget] per_day_usd` and `per_story_usd` only bound what one process spends; only the pause file, written after a trip, carries anything across processes. The docs state this limit: docs/guide/how-models.md (the honest-status note) and docs/threat-model.md (T16).

### Steps to reproduce

1. In a scratch checkout with `[budget] per_day_usd = 1`, run one `aayu` command whose direct model calls cost 0.60 USD (for example `triage` against a fake model endpoint).
2. Run a second such command in the same checkout on the same day.
3. Observe whether the second command trips the breaker.

### Expected behaviour

The breaker counts every model call recorded in the checkout's actions log: today's records for the per-day budget and the story's records for the per-story budget, so the call that brings the day's total to the limit trips the breaker whichever process makes it.

### Actual behaviour

Found by reading the code, not yet by a live run: `rg '\.Seed\('` finds no caller. The second process starts at 0 USD, reaches 0.60 USD, and does not trip, although the day's total is 1.20 USD against a 1 USD limit.

### Completion Goal

1. [api-trace] state: two `aayu` processes in one checkout on the same day, each making model calls that cost 0.60 USD, with `[budget] per_day_usd = 1`; expected: the second process's call trips the breaker: it exits 6, the pause file is written, and one page names the day's spend
2. [api-trace] state: two processes in one checkout spending 0.60 USD each on the same story, with `[budget] per_story_usd = 1` and a high per-day limit; expected: the second process trips the breaker for that story with exit 6
3. [api-trace] state: an actions log holding 0.90 USD of model calls from yesterday and a new process today with `[budget] per_day_usd = 1`; expected: a 0.60 USD call today does not trip the breaker

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Run two processes in one scratch checkout against a fake model endpoint with priced replies, once on the same day, once on the same story, and once with yesterday's spend in the log, and record each exit code, the pause file, and the pages sent.

### Environment

local scratch checkout with a fake model endpoint and synthetic spend; never production
````
