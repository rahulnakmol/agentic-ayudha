# Vague: performance (`perf`)

Qualifiers: fast, faster, fastest, quick, quickly, snappy, speedy, instant, instantly, performant, lightning,
slow, sluggish, laggy, lag, efficient

"Fast" means nothing until it names an operation, a percentile, a number, and the conditions it holds under. Ask
about these dimensions in order and cite the facts: today's routes, tests, configured budgets, and the budgets that
are missing.

## Dimensions

### Which operation (`perf.operation`)

- Ask: Which exact action must be fast: the first page load, typing in the box, the results appearing, opening a
  result? From whose point of view?
- Default: the one action the request names, measured from the user's input to the visible result, because that is
  what users feel.
- Evidence: metric
- Facts: route, code

### Number and percentile (`perf.target`)

- Ask: What number must it meet, and at which percentile (p50 for typical, p95 for most users)? Is there a number
  it is today?
- Default: p95, with the number the human gives; when they have none, offer today's figure minus a margin as a
  proposal to confirm, never as a fact.
- Evidence: metric
- Facts: floor, gap

### Conditions (`perf.conditions`)

- Ask: Under what conditions must it hold: data size (how many items to search), device and network, concurrent
  users, cold or warm cache?
- Default: the seeded data set and a mid-range laptop on a local network, because proof captures run on seeded data
  only.
- Evidence: metric
- Facts: code, test

### How it is measured (`perf.measure`)

- Ask: Where is the number measured: a benchmark test, an API trace, Lighthouse, production telemetry? Which test
  should prove it?
- Default: a repeatable benchmark or API trace on the seeded data, recorded as a `metric` criterion, because
  production telemetry cannot be captured as proof.
- Evidence: metric
- Facts: test, event

### Perceived speed (`perf.perceived`)

- Ask: While the user waits, what do they see: results as they type, a spinner, a skeleton? Is a partial result
  acceptable?
- Default: keep today's loading feedback; a new loading state only when the human asks for one.
- Evidence: recording
- Facts: gap, route

### Guarding against regressions (`perf.regression`)

- Ask: Should a budget in CI stop it from getting slow again (a k6 threshold or a Lighthouse budget)?
- Default: none in this story; propose it as a parked idea, because a new CI budget is its own decision.
- Evidence: metric
- Facts: floor, gap
