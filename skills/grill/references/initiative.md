# grill initiative: question set

Keyed to the Initiative sections of the work-item schema (`internal/workitems/schema.go`, schema v1). Ask in this
order. Each answer becomes the section of the same name (`### <Label>` in the body; the field ID in backticks is the
key in a JSON draft). Stop when every required field has an answer the human confirmed. Ask; do not invent: what the
human leaves open goes into an open question, not into the item.

## Create

### Summary (`summary`, required)

- What bet are we making, and which Goal does it serve? (the `--parent` Goal; an Initiative needs one at Ready)

### Hypothesis (`hypothesis`, required)

- If we do X, the Goal's metric Y moves by Z because W. Which part is the riskiest assumption?

## Ready

### Metric (`metric`, required)

- Which number moves if the bet works? Same forms as a Goal: `<source>:<metric> [filter]` or a snapshot metric.

### Baseline (`baseline`, required)

- For a provider metric the binary fetches it (28 days before today) and writes the tuple; for a snapshot metric,
  the measured value and its snapshot. No baseline, no target.

### Target (`target`, required)

- What value, by when?
