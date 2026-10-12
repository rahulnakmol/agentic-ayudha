# Rubric: brief, eli5 register, explain purpose

Score the brief from 1 to 5. It explains Feature #7 (CSV export of filtered claims) to a curious ten-year-old.

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

Register and purpose: everyday words and short sentences (the Flesch-Kincaid grade was already checked to be 6 or
lower); technical ideas are explained with a concrete everyday picture rather than jargon; the reader learns what
changed, why it helps, and how we know it works. Simplifying is fine; changing a fact is not (for example, rounding
4.2 to "about four" is fine, "five" is not).
