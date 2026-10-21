# Rubric: tell, how-to guide for exporting filtered claims

Score the guide from 1 to 5. It is a how-to page (docs/export.md) for claims reviewers: export the claims they have
filtered to a CSV file.

The sources are the only truth: `aayu tell`'s brief, `design/brand.yaml`, and the grounding. Before you see the guide,
`aayu tell --from`'s deterministic checks passed: every link to a work item is a proven Feature, no sentence names an
unproven Feature, every proven Feature the copy names is linked, no avoided word, no superlative. Do not re-check
those; judge whether each claim traces to a pack, the voice, and whether a reader can do the task.

A claim is any statement of what the product does, a control, a step, a result, or a number. A claim traces to a pack
when it restates a criterion of Feature #7's authoritative pack P1 whose inspect verdict is met (its state and
expected result, or the inspect judgement), a floor that passed, stated as exactly what it checked, the Feature's
summary, problem, or scope, or its Result in its own unit and population. The controls the sources name
are the Columns picker, Export CSV, and the list toolbar; the file name `claims.csv` comes from an inspect judgement.
Anything else is an unsupported claim: a menu, button, dialog, format choice, setting, or step no source names; a
limit, duration, row count, or file size no source gives; a behaviour beyond a criterion (criterion 3 shows the list
keeps responding on a list of more than 10,000 claims, not that a file of that size is complete or fast); an
accessibility promise beyond the passing floors. Links to the pages the grounding lists under `docs` are fine; links to
other pages or URLs are invented.

A how-to guide serves one task: a title that names it, what the reader needs first, numbered steps in order, the result
to expect, and what is outside the Feature's scope (PDF, scheduled exports, other lists), linking the Feature (#7).

- 5: Every claim traces to the pack or the Feature's fields, with nothing invented; a reader can export a filtered list
  by following the steps; the voice is warm, encouraging, direct, and in the second person, as `design/brand.yaml`
  asks, and jargon (CSV) is explained the first time it appears.
- 4: As 5, with one minor lapse: one step less precise than its criterion, one off-tone sentence, or jargon left
  unexplained once.
- 3: Usable, but one claim is unsupported (an invented control, step, or limit), or the voice slips in several places.
- 2: Several unsupported claims, or the steps do not get a reader to the file.
- 1: Not usable.
