# rahulnakmol/agentic-ayudha#503: UX floors: default page-speed budgets, a visual-regression floor at 0.1 %, and a blocking skipped: web

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `5decd018e43039c3efd6f3f2a471213fdd55a0691967d2ed2a629e16269e0aaf` |
| Completion Goal hash | `af7a46def4dc5d0875c31d9756aa96dbc6d818698a347668f4de89704418913a` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/503/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `CONSTRAINTS.md` and `aayu config show` on a repository with no perf keys; then a stand-in Lighthouse that reports, for one state over 3 runs, an LCP of 1800 ms, 9000 ms and 2000 ms; then 1800 ms, 9000 ms and 8000 ms; expected: the defaults read `min_score = 0.9`, `max_lcp_ms = 2500`, `max_cls = 0.1`, `max_tbt_ms = 200`, `runs = 3`, `layers = ["L01"]`; the first state passes on the median of 2000 ms; the second fails on the median of 8000 ms; the check records all three runs
2. [api-trace] state: a baseline capture and a head capture of a `[review] baseline` path at 1280 by 720 that differ by one removed 40 by 40 pixel button; the same with `max_diff_percent = 2`; a baseline path that is also one of the story's Completion Goal states; and no baseline within 20 first-parent commits; expected: the removed button fails the visual floor at 0.1 % and passes at 2 %; the Completion Goal path is excluded and named as excluded; with no baseline the check reads `skipped: visual (no baseline)` and does not block
3. [api-trace] state: `[verify] platforms = ["web"]`, a footprint with L01, and no `[verify.environments.local] url`; then the same footprint without L01; then `platforms = ["cli"]` with L01; expected: the first records `skipped: web` for the a11y, perf and visual floors, which blocks ship until a floor waiver names web; the second and third skip those floors without blocking
4. [api-trace] state: a perf or visual miss with the clock before 2026-11-14, then on 2026-11-14; expected: before the date the check passes with a detail that says advisory until 2026-11-14 and names the miss; on the date it fails
5. [api-trace] state: `aayu ship` of a story with `[review] baseline` paths and a `[proof] media` store, against the fake GitHub; then without a media store; expected: after the merge ship captures each baseline path for the merge commit and writes `baselines/<sha>/` to the ledger with the media in the store; without a store it skips the capture and says why

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As a product team using aayu, I want a story that changes the UI to prove it is still fast and still looks as it did where it was not meant to change, so that a slow page or a vanished button cannot pass because nobody set a budget or a baseline. Decision #480 (accepted, option 1, section 2) gives the perf floor default budgets (score 0.9, LCP 2500 ms, CLS 0.1, TBT 200 ms, the median of 3 Lighthouse runs) for L01, adds a `visual` floor that compares each `[review] baseline` path with the nearest baseline at 0.1 % of pixels, and adds a blocking `skipped: web` when a web story touches L01 and verify has no target to capture. `ship` captures a baseline after merge when a baseline list and a media store are set. Misses are advisory until `[verify] new_floors_blocking_from` (2026-11-14), then they fail. The ui-review model lane stays a review, not a floor.

### Completion Goal

1. [api-trace] state: `CONSTRAINTS.md` and `aayu config show` on a repository with no perf keys; then a stand-in Lighthouse that reports, for one state over 3 runs, an LCP of 1800 ms, 9000 ms and 2000 ms; then 1800 ms, 9000 ms and 8000 ms; expected: the defaults read `min_score = 0.9`, `max_lcp_ms = 2500`, `max_cls = 0.1`, `max_tbt_ms = 200`, `runs = 3`, `layers = ["L01"]`; the first state passes on the median of 2000 ms; the second fails on the median of 8000 ms; the check records all three runs
2. [api-trace] state: a baseline capture and a head capture of a `[review] baseline` path at 1280 by 720 that differ by one removed 40 by 40 pixel button; the same with `max_diff_percent = 2`; a baseline path that is also one of the story's Completion Goal states; and no baseline within 20 first-parent commits; expected: the removed button fails the visual floor at 0.1 % and passes at 2 %; the Completion Goal path is excluded and named as excluded; with no baseline the check reads `skipped: visual (no baseline)` and does not block
3. [api-trace] state: `[verify] platforms = ["web"]`, a footprint with L01, and no `[verify.environments.local] url`; then the same footprint without L01; then `platforms = ["cli"]` with L01; expected: the first records `skipped: web` for the a11y, perf and visual floors, which blocks ship until a floor waiver names web; the second and third skip those floors without blocking
4. [api-trace] state: a perf or visual miss with the clock before 2026-11-14, then on 2026-11-14; expected: before the date the check passes with a detail that says advisory until 2026-11-14 and names the miss; on the date it fails
5. [api-trace] state: `aayu ship` of a story with `[review] baseline` paths and a `[proof] media` store, against the fake GitHub; then without a media store; expected: after the merge ship captures each baseline path for the merge commit and writes `baselines/<sha>/` to the ledger with the media in the store; without a store it skips the capture and says why

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Run the perf and visual floors with a stand-in Lighthouse and recorded PNG captures for a base and a head commit, across the footprint, platform, target, baseline and date combinations above. Run ship against the fake GitHub with a stand-in capture and a local media store.

### Environment

local checkout and this repository's CI build job on a pull request; the fake GitHub in internal/github/fake; the real Playwright and Lighthouse only in the floors workflow; never production

### Links

depends-on #496

### Checkpoints

1. `go test ./internal/verify -run 'TestPerfDefaults|TestVisualFloor|TestSkippedWeb'` shows the median, the 0.1 % limit, the exclusion, the blocking skip, and the advisory date (criteria 1, 2, 3, 4)
2. `go test ./internal/cli -run 'TestShipCapturesBaseline'` (criterion 5)

### Assumptions

- a baseline and a head capture come from the same pinned Chromium, so no renderer tolerance is needed
- "nearest baseline" is the nearest first-parent ancestor of the base with `baselines/<sha>/` on the ledger, within 20 commits
- `verify.new_floors_blocking_from` is 2026-11-14, shared by every floor Decision #480 added, and lands with #496

### Open questions

- deferred: until a product sets a media store, no baseline exists and the visual floor skips without blocking; Decision #480 calls this the soft spot of this family

### Provenance

Request: "in proof floors should talk about code quality and user experience if there is a UI, or developer experience if it's an API or CLI. Goal: proof floors that, chosen by the story's layer footprint, cover: User experience, when the footprint has a UI (L01). UX: a default Lighthouse budget for L01 stories, and visual regression against the baseline using the existing review.figma_diff_max and baseline settings. Ground rule 7 still holds: a floor is deterministic, and no model opinion can be a floor. The Gemini ui-review lane stays a review, not a floor. It should list each floor, its default, the layers it applies to, the tool pinned by SHA-256, and what skipped means. Build only after it is accepted, one story per floor family, each with tests that a plausible wrong implementation would fail. Start the stories outside DAST now under #480's acceptance (code quality, UX, API and CLI developer experience)."
Term: "UI" → the product's user interface, layer L01; "User experience, when the footprint has a UI (L01)"
Term: "UX" → user experience; "UX: a default Lighthouse budget for L01 stories"
Term: "L01" → the Experience layer in LAYERS.md; "when the footprint has a UI (L01)"
Term: "API" → a product's HTTP or RPC interface; "developer experience if it's an API or CLI"
Term: "CLI" → a product's command-line interface; "developer experience if it's an API or CLI"
Term: "SHA-256" → the hash each downloaded tool is checked against; "the tool pinned by SHA-256"
Term: "DAST" → the ZAP scan of the running app, built in #496; "Start the stories outside DAST now"
Term: "LCP" → largest contentful paint, a Lighthouse metric; "a default Lighthouse budget for L01 stories"
Term: "CLS" → cumulative layout shift, a Lighthouse metric; "a default Lighthouse budget for L01 stories"
Term: "TBT" → total blocking time, a Lighthouse metric; "a default Lighthouse budget for L01 stories"
1. stated (request): "a default Lighthouse budget for L01 stories"
2. stated (request): "visual regression against the baseline using the existing review.figma_diff_max and baseline settings"
3. stated (request): "User experience, when the footprint has a UI (L01)" / "what skipped means"
4. stated (request): "Start the stories outside DAST now under #480's acceptance"
5. stated (request): "visual regression against the baseline"
````
