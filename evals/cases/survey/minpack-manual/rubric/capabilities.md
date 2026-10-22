# Rubric: survey of a legacy Fortran library with its 1980 user guide: the capabilities section

This section holds the capability claims.

The deterministic checks have passed on this section and the sections before it: every citation resolves (the
guide's lines are cited as `manual:anl-80-74.pdf.txt:<line>`) and each claim's basis is allowed. Judge what they
cannot: whether the claims are true of the code, the guide, and the probes shown, useful, and honest about what is
unknown. Compare every claim you can with the files and guide lines the host read; a claim they contradict is a
fabrication, and so is a claim a cited guide line does not say. Judge this section only; the sections it rests on
are shown for context and were judged on their own.

Score 5 when every part of the criterion below holds, 4 when one part is weak, 3 when two are weak or one is
missing, 2 when the section would mislead a reader on any point, and 1 when it contains a fabrication (a claim the
cited code does not support, a user or customer the repository never names, or a number nothing measures).

1. **The capability map rests on code and guide.** Each capability is what a caller can do, in plain words, one per
   claim, cites the routine that does it, and cites the guide's description where the guide has one; a routine the
   guide does not document (covar) is said to be undocumented, and nothing the guide describes but the checkout
   lacks is claimed as a capability.
