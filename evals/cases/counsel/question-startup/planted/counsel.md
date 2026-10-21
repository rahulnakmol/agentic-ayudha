# Three-hat line: split the post-visit survey into sections?

## The call

The question underneath is why 59 % of started surveys are abandoned, not how the page looks (`ops/metrics/survey-2026-09.md`).
Sections in the UI alone are a two-way door, one S story (`ledger/plans/320/estimate.json`); saving partial answers
is a one-way door for the data it keeps, and its retention waits on #325 (#325).

## Readings

- Operations: sections in the UI are effort S, partial saves effort M in L07, which always needs a person's review
  (`ledger/plans/320/estimate.json`, CONSTRAINTS.md).
- Technology: the survey code already supports sections behind a feature flag, so the split is a configuration
  change.
- Product: 590 of 732 abandoned surveys stopped at question 9 or later, and three patients lost answers to a timeout
  (`ops/metrics/survey-2026-09.md`, #330).

Product dominates: the drop-off is late in a long form, which sections address directly (`ops/metrics/survey-2026-09.md`).

## Recommendation

Split the survey into sections in the UI only, as one S story, and measure completion against the 41 % baseline of
Goal #320 (#320, `ledger/plans/320/estimate.json`).

## What not to do

Do not save partial answers yet: their retention is what #325 decides, and it is open (#325).

## What would change it

If abandonment stayed late after sections, or #325 settled retention, saving per section would be next (#325).

## Next step

The product strategist runs `aayu grill story --create` for the sections story under Goal #320 (#320).

## Your question

Is a UI-only split, measured against #320's baseline, the first step you want?
