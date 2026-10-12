# grill feature: question set

Keyed to the Feature sections of the work-item schema (`internal/workitems/schema.go`, schema v1). Ask in this
order. Each answer becomes the section of the same name (`### <Label>` in the body; the field ID in backticks is the
key in a JSON draft). Stop when every required field has an answer the human confirmed. Ask; do not invent: what the
human leaves open goes into an open question, not into the item.

## Create

### Summary (`summary`, required)

- What capability do users get? Which Initiative does it serve? (the `--parent`; a Feature needs one at Ready)

### Problem (`problem`, required)

- What user problem does it solve, and what evidence shows the problem (support tickets, analytics, research)?

## Ready

### Scope (`scope`, required)

- What is in scope, and what is explicitly out of scope?

### Expected paths (`expected_paths`, optional)

- Which paths will change? One glob per line (`internal/claims/**`, `web/src/claims/**`). `slice` computes the layer
  footprint from them through `LAYERS.md`.

### Links (`links`, optional)

- Does it depend on another Feature? One line each: `depends-on #N`.
