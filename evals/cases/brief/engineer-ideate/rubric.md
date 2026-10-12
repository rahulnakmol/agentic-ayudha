# Rubric: brief, engineer register, ideate purpose

Score the brief from 1 to 5. It gives engineers the evidence on Feature #7 (CSV export of filtered claims) and ideas
for what to do next.

The grounding is the only source. A claim is any statement of fact about the work: what changed, a number, a date, a
floor result, an outcome. Before you see the brief, `aayu brief`'s deterministic checks passed: structure (title, then
the takeaway), every citation names a real source, the register's lint (below), and visuals. Do not re-check those; judge
grounding, fit, and usefulness.

Inferences: anything beyond what the floors, the criterion map, or the Result state is an inference. One the brief
labels as such ("this suggests …") is not a claim when it follows from the sources; one stated as fact is an
unsupported claim. A UI element, control, or step that no source names is an invented claim.
Absence of evidence is written as "the evidence does not show …", never as "not done", "not possible", "untested" or
"can't". Out-of-scope items are written as "outside this story's scope". A measure shows only what it counts: name
the count, not the behaviour behind it. Writing absence as fact ("untested", "can't be saved") or a count as a behaviour ("this
shows that people use the file") is an unsupported claim.
A count keeps its own unit (exports, not "files saved"), and out-of-scope items are "outside this story's scope".

- 5: Every claim matches what its cited source says, with nothing invented (no numbers, dates, names, causes, or
  outcomes beyond the grounding); the takeaway is first and correct; the register and purpose fit as described below.
- 4: As 5, with one minor lapse: a citation that could be more precise, or one sentence slightly off register.
- 3: Usable, but one claim is unsupported or overstated, or the register slips in several places.
- 2: Several unsupported or overstated claims, or the brief misses its purpose.
- 1: Not usable.

Register and purpose: precise and compact, naming floors, values, and thresholds exactly as the pack gives them. An
 section lists at least two ideas. Each idea is plainly a proposal, not a claim, and is motivated by
something in the grounding (for example the skipped screen-reader floor, the recording-only evidence for
responsiveness, the out-of-scope list, or the Result). Ideas presented as facts, or ideas with no link to the evidence,
lower the score.
