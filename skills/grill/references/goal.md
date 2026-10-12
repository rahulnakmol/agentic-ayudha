# grill goal: question set

Keyed to the Goal sections of the work-item schema (`internal/workitems/schema.go`, schema v1). Ask in this
order. Each answer becomes the section of the same name (`### <Label>` in the body; the field ID in backticks is the
key in a JSON draft). Stop when every required field has an answer the human confirmed. Ask; do not invent: what the
human leaves open goes into an open question, not into the item.

## Create

### Summary (`summary`, required)

- What outcome do we want, and who benefits from it?

## Ready

### Beneficiaries (`beneficiaries`, required)

- Which beneficiary groups (slugs from `[beneficiaries]` in config) does it serve?

### Metric (`metric`, required)

- Which number tells us the outcome moved? Write it so `aayu measure` can re-run it: a provider baseline as
  `<source>:<metric> [filter]` (`posthog:users event = 'signed_up'`, `sentry:crash_free_sessions`,
  `github:prs_merged`), or a snapshot metric from exports (`acquisition`, `activation`, `retention`, `referral`,
  `revenue`, `north_star`).

### Baseline (`baseline`, required)

- Where is the number today? For a provider metric, leave this to the binary: `aayu grill goal` fetches it through
  the integration over the 28 days before today and writes the value, population, window, source query, and fetch
  time. For a snapshot metric, give the value `aayu measure` reported and its snapshot.
- No baseline, no target: if the metric cannot be reached, the Goal is refused Ready, naming the metric.

### Target (`target`, required)

- What value, by when? Use a comparator when the direction matters (`>= 0.40 by 2027-03-31`).
