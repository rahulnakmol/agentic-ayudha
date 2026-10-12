# grill decision: question set

Keyed to the Decision sections of the work-item schema (`internal/workitems/schema.go`, schema v1). Ask in this
order. Each answer becomes the section of the same name (`### <Label>` in the body; the field ID in backticks is the
key in a JSON draft). Stop when every required field has an answer the human confirmed. Ask; do not invent: what the
human leaves open goes into an open question, not into the item.

## Create

### Question (`summary`, required)

- What must be decided, and why now? Which item does it belong to, if any? (the `--parent`)

## Ready (Decision-ready)

### Options (`options`, required)

- Which options are on the table, including doing nothing? One per line.

### Evaluation (`evaluation`, required)

- How does each option score against the criteria that matter (cost, risk, reversibility, user impact)?

### Recommendation (`recommendation`, required)

- Which option do you recommend, and why?

### Reversible (`reversible`, required)

- `true` or `false`: can the choice be undone cheaply? Only a reversible Decision's recommendation applies
  provisionally when its SLA lapses (V8).

### Links (`links`, optional)

- Does it supersede an earlier Decision (`supersedes #N`), constrain items (`constrains #N`), or waive a floor on a
  story (`waives #N`)?
