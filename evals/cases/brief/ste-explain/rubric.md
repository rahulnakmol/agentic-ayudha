# Rubric: brief, ste register, explain purpose

Score the brief from 1 to 5. It explains Feature #7 (CSV export of filtered claims) in Simplified Technical English.

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

- 5: Every claim matches what its cited source says, with nothing invented (no numbers, dates, names, causes, or
  outcomes beyond the grounding); the takeaway is first and correct; the register and purpose fit as described below.
- 4: As 5, with one minor lapse: a citation that could be more precise, or one sentence slightly off register.
- 3: Usable, but one claim is unsupported or overstated, or the register slips in several places.
- 2: Several unsupported or overstated claims, or the brief misses its purpose.
- 1: Not usable.

Register and purpose: the STE lint already passed (sentences of at most 20 words, approved words, one instruction per
sentence). Judge the rest of STE: active voice, one topic per sentence, instructions that start with a verb and are
correct for the Feature as the grounding describes it, no ambiguity. The reader learns what the export does, how to
use it as far as the grounding says, and what proves it. Steps the grounding does not support (for example a menu or
a setting it never mentions) are invented claims.
