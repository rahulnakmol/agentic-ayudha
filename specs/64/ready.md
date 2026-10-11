# rahulnakmol/agentic-ayudha#64: WP7.3 `brief` core

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `1e5a075f74f29f1222fb0940576a6b311c2895f9b7fd30882a0c41a829300e00` |
| Completion Goal hash | `94ee2f76823c3738bf5b75ac04c254894b123e56e54605249ce062cac6ba2472` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/64/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the WP7.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: register evals, tier 2 at the WP3.4 bar per register, with the `eli5` rubric additionally requiring reading grade ≤ 6 (Flesch-Kincaid) and `ste` requiring the STE lint to pass (sentence ≤ 20 words, approved-word list, one instruction per sentence)
2. [api-trace] state: the WP7.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: planted 30-word sentence fails STE lint
3. [api-trace] state: the WP7.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: every chart carries a query and re-running the query reproduces the chart data (golden)
4. [api-trace] state: the WP7.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: HTML output has zero serious/critical axe findings and every image has non-empty alt text
5. [api-trace] state: the WP7.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `brief close` on a story without an authoritative pack exits 4

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP7.3 `brief` core built as docs/BUILD-PLAN.md specifies, so that M7 (Loops and explain) delivers: Scheduled loops, measure, brief, tell, audit and retro, evolve, release.

Plan text (docs/BUILD-PLAN.md, WP7.3): Registers `eli5|ste|engineer|regulatory`; purposes `explain|ideate|innovate|close`; HTML artifact with grounding footer; STE100 linter; a11y check on the HTML; `close` refuses without authoritative pack and Result.

### Completion Goal

1. [api-trace] state: the WP7.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: register evals, tier 2 at the WP3.4 bar per register, with the `eli5` rubric additionally requiring reading grade ≤ 6 (Flesch-Kincaid) and `ste` requiring the STE lint to pass (sentence ≤ 20 words, approved-word list, one instruction per sentence)
2. [api-trace] state: the WP7.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: planted 30-word sentence fails STE lint
3. [api-trace] state: the WP7.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: every chart carries a query and re-running the query reproduces the chart data (golden)
4. [api-trace] state: the WP7.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: HTML output has zero serious/critical axe findings and every image has non-empty alt text
5. [api-trace] state: the WP7.3 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `brief close` on a story without an authoritative pack exits 4

### Measure

WP7.3 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP7.3)

### Threshold

all 5 criteria

### Scenario

Open the WP7.3 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
