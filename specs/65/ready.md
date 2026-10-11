# rahulnakmol/agentic-ayudha#65: WP7.4 `brief` visuals

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `fb12bd34900c2084236c408a1413c1e19b816deba74eb190cb67f5589a8aeaa8` |
| Completion Goal hash | `1fb1b177c34bc48ee1ab6e3b0115d4b62458126f119220ecd27087776fbafd83` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/65/ready.md` |

Completion Goal at Ready:

1. [screenshot] state: the WP7.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: register evals, tier 2 at the WP3.4 bar per register, with the `eli5` rubric additionally requiring reading grade ≤ 6 (Flesch-Kincaid) and `ste` requiring the STE lint to pass (sentence ≤ 20 words, approved-word list, one instruction per sentence)
2. [screenshot] state: the WP7.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: planted 30-word sentence fails STE lint
3. [screenshot] state: the WP7.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: every chart carries a query and re-running the query reproduces the chart data (golden)
4. [screenshot] state: the WP7.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: HTML output has zero serious/critical axe findings and every image has non-empty alt text
5. [screenshot] state: the WP7.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `brief close` on a story without an authoritative pack exits 4

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As the founder, I want WP7.4 `brief` visuals built as docs/BUILD-PLAN.md specifies, so that M7 (Loops and explain) delivers: Scheduled loops, measure, brief, tell, audit and retro, evolve, release.

Plan text (docs/BUILD-PLAN.md, WP7.4): `*.diagram` → SVG (static + interactive), charts from snapshots with their query, illustrations via Painter/Gemini labelled as such, annotated screens from packs; alt text states the takeaway.

### Completion Goal

1. [screenshot] state: the WP7.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: register evals, tier 2 at the WP3.4 bar per register, with the `eli5` rubric additionally requiring reading grade ≤ 6 (Flesch-Kincaid) and `ste` requiring the STE lint to pass (sentence ≤ 20 words, approved-word list, one instruction per sentence)
2. [screenshot] state: the WP7.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: planted 30-word sentence fails STE lint
3. [screenshot] state: the WP7.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: every chart carries a query and re-running the query reproduces the chart data (golden)
4. [screenshot] state: the WP7.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: HTML output has zero serious/critical axe findings and every image has non-empty alt text
5. [screenshot] state: the WP7.4 acceptance check runs on its pull request (tests in CI; live parts behind AAYU_LIVE=1); expected: `brief close` on a story without an authoritative pack exits 4

### Measure

WP7.4 acceptance criteria observed (docs/BUILD-PLAN.md, the 'Accept' line of WP7.4)

### Threshold

all 5 criteria

### Scenario

Open the WP7.4 pull request to dev; CI runs build, tests, lint, security, docs, generate-drift, and release-snapshot; the acceptance tests and any live check named in the plan run and their output is the evidence.

### Environment

GitHub Actions on the pull request, and rahulnakmol/agentic-ayudha or aayu-scratch-* repositories with synthetic fixtures only
````
