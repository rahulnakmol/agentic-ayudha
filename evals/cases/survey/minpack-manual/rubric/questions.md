# Rubric: survey of a legacy Fortran library with its 1980 user guide: the questions section

This section holds the questions and the summary.

The deterministic checks have passed on this section and the sections before it: every citation resolves (the
guide's lines are cited as `manual:anl-80-74.pdf.txt:<line>`) and each claim's basis is allowed. Judge what they
cannot: whether the claims are true of the code, the guide, and the probes shown, useful, and honest about what is
unknown. Compare every claim you can with the files and guide lines the host read; a claim they contradict is a
fabrication, and so is a claim a cited guide line does not say. Judge this section only; the sections it rests on
are shown for context and were judged on their own.

Score 5 when every part of the criterion below holds, 4 when one part is weak, 3 when two are weak or one is
missing, 2 when the section would mislead a reader on any point, and 1 when it contains a fabrication (a claim the
cited code does not support, a user or customer the repository never names, or a number nothing measures).

1. **Questions are the right ones.** Few, ordered by how much the answer changes the port: which callers and entry
   points, how exact the results must be, and whether the code or the guide is the reference where they disagree,
   each with why it matters and a default with a reason. No question is answered by the skill.
