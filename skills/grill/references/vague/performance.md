# Vague: performance (`perf`)

Qualifiers: fast, faster, fastest, quick, quickly, snappy, speedy, instant, instantly, performant, lightning,
slow, sluggish, laggy, lag, efficient

"Fast" means nothing until it names an operation, a percentile, a number, and the conditions it holds under. Ask
about these dimensions in order and cite the facts: today's routes, tests, configured budgets, and the budgets that
are missing.

## Dimensions

### Which operation (`perf.operation`)

- Ask: Which exact action must be fast, from whose point of view? For a web app: the first page load, typing in a
  box, results appearing. For a command-line tool: a command from launch to its output. For an API: one endpoint.
  For a library: one call.
- Default: the one action the request names, measured from the user's input to the visible result, because that is
  what users feel.
- Evidence: metric
- Facts: route, code
- Topics: action, viewpoint

### Interaction latency (`perf.interaction`)

- Ask: From which input to which visible change: a click to the next screen, a keystroke to results, the first page
  load (LCP)? Is Interaction to Next Paint the right measure, or a mark of your own?
- Default: input to the visible result on the seeded data, measured in the browser, because that is what users feel.
- Evidence: metric
- Facts: route, floor
- Products: web
- Topics: input-to-paint, page-load

### Startup and per-command time (`perf.startup`)

- Ask: How long may the tool take from launch to its first output, and how long may each named command take at p95,
  cold (first run after install) and warm?
- Default: no slower than today for every command, measured as repeated runs on the seeded data; a number only when
  the human gives one.
- Evidence: metric
- Facts: command, product
- Products: cli
- Topics: cold-start, per-command

### Endpoint latency (`perf.endpoint`)

- Ask: Which endpoint, at what percentile and number, under what request rate and payload size?
- Default: p95 on the seeded data at the request rate the human names, shown by an API trace or a load test.
- Evidence: api-trace
- Facts: route, floor, gap
- Products: api
- Topics: endpoint, request-rate

### Call cost (`perf.library`)

- Ask: Which call, with what input size: how many operations per second, or how long per call, and how many
  allocations?
- Default: a benchmark of the named call on a fixed input, compared with today's result.
- Evidence: metric
- Facts: code, test
- Products: library
- Topics: call, allocations

### Number and percentile (`perf.target`)

- Ask: What number must it meet, and at which percentile (p50 for typical, p95 for most users)? Is there a number
  it is today?
- Default: p95, with the number the human gives; when they have none, offer today's figure minus a margin as a
  proposal to confirm, never as a fact.
- Evidence: metric
- Facts: floor, gap
- Topics: percentile-and-number, current-value

### Conditions (`perf.conditions`)

- Ask: Under what conditions must it hold: data size (how many items to search), device and network, concurrent
  users, cold or warm cache?
- Default: the seeded data set and a mid-range laptop on a local network, because proof captures run on seeded data
  only.
- Evidence: metric
- Facts: code, test
- Topics: data-size, device-and-network, cache-state, concurrency

### How it is measured (`perf.measure`)

- Ask: Where is the number measured: a benchmark test, an API trace, Lighthouse, production telemetry? Which test
  should prove it?
- Default: a repeatable benchmark or API trace on the seeded data, recorded as a `metric` criterion, because
  production telemetry cannot be captured as proof.
- Evidence: metric
- Facts: test, event
- Topics: proof, test

### Perceived speed (`perf.perceived`)

- Ask: While the user waits, what do they see: results as they type, a spinner, a skeleton, a progress line on the
  terminal? Is a partial result acceptable?
- Default: keep today's loading feedback; a new loading state only when the human asks for one.
- Evidence: recording
- Facts: gap, route
- Topics: loading-feedback, partial-results

### Guarding against regressions (`perf.regression`)

- Ask: Should a budget in CI stop it from getting slow again (a k6 threshold or a Lighthouse budget)?
- Default: none in this story; propose it as a parked idea, because a new CI budget is its own decision.
- Evidence: metric
- Facts: floor, gap
- Topics: ci-budget
