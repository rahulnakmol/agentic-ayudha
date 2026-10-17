# Rubric: tell, release notes for v1.5.0

Score the release notes from 1 to 5. They tell the claims teams who use Benefits QA what changed in v1.5.0.

The sources are the only truth: `aayu tell`'s brief, `design/brand.yaml`, and the grounding. Before you see the notes,
`aayu tell --from`'s deterministic checks passed: every link to a work item is a proven Feature, no sentence names an
unproven Feature, every proven Feature the copy names is linked, no avoided word, no superlative. Do not re-check
those; judge whether each claim traces to a pack, the voice, and usefulness.

A claim is any statement of what the product does, what changed, who it helps, or a number. A claim traces to a pack
when it restates a criterion of an authoritative pack whose inspect verdict is met (its state and expected result, or
the inspect judgement), a floor that passed, stated as exactly what it checked, the Feature's summary, problem, or
scope, or a Result in its own unit and population. Anything else is an unsupported claim: a control,
step, or behaviour no criterion names; a number, date, or count no source gives; a benefit stated as
measured (neither #15 nor #16 has a Result yet); a change described as part of v1.5.0 that the grounding lists under
another release (#7 shipped in v1.4.0).

What shipped in v1.5.0: stories #18 and #19 (Feature #15 Saved filters) and #22 (Feature #16 Claim notes), which are
proven; story #21 under #9 Bulk reassignment, which has no authoritative pack; and bug #26, which has no Feature.
Release notes describe only proven Features. The other two changes are counted (two), never described or named;
describing #21's or #26's change in any words is an unsupported claim. Leaving the count out entirely is a minor
lapse: the reader should know the release holds more than the notes describe.

- 5: Every claim traces to a pack or the Feature's fields, with nothing invented; each proven Feature that shipped is
  covered with what changed, who it helps, and its link (#15, #16); the count of the other changes is right; the voice
  is plain, precise, and calm, as `design/brand.yaml` asks, with no marketing adjective or avoided phrase.
- 4: As 5, with one minor lapse: one slightly promotional or vague sentence, a missing count, or one detail less
  precise than its criterion.
- 3: Usable, but one claim is unsupported or overstated, or the voice slips in several places.
- 2: Several unsupported claims, or the notes describe a change from an unproven Feature or the bug.
- 1: Not usable.
