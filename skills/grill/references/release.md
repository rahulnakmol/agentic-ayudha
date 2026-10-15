# grill release: question set

Keyed to the Release sections of the work-item schema (`internal/workitems/schema.go`, schema v1). Ask in this
order. Each answer becomes the section of the same name (`### <Label>` in the body; the field ID in backticks is the
key in a JSON draft). Stop when every required field has an answer the human confirmed. Ask; do not invent: what the
human leaves open goes into an open question, not into the item.

## Create

### Summary (`summary`, required)

- What does this release ship, and to whom? Which Goal does it serve, if any? (the `--parent`)

### Version (`version`, required)

- A rolling snapshot timestamp or `vX.Y.Z`.

## Ready

### Links (`links`, optional)

- Which Features, Stories, and Bugs does it include? One line each: `includes #N`.
