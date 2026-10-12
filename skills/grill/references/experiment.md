# grill experiment: question set

Keyed to the Experiment sections of the work-item schema (`internal/workitems/schema.go`, schema v1). Ask in this
order. Each answer becomes the section of the same name (`### <Label>` in the body; the field ID in backticks is the
key in a JSON draft). Stop when every required field has an answer the human confirmed. Ask; do not invent: what the
human leaves open goes into an open question, not into the item.

## Create

### Hypothesis (`summary`, required)

- If we change X for users behind a flag, metric Y moves by Z. Which Feature or Initiative is it under? (the
  `--parent`; needed at Ready)

## Ready

### Metric (`metric`, required)

- Which conversion event decides it?

### Baseline (`baseline`, required)

- What is the conversion rate today, from which query? (`aayu experiment` fetches it when it files the Experiment.)

### Flag (`flag`, required)

- Which feature flag key gates the change?

### Minimum sample (`min_sample`, required)

- How many users per arm, with the power calculation (alpha, power, baseline, detectable rate)?
