# Rubric: survey of a legacy Fortran library with its 1980 user guide: the engineering section

This section holds the engineering claims and the traps.

The deterministic checks have passed on this section and the sections before it: every citation resolves (the
guide's lines are cited as `manual:anl-80-74.pdf.txt:<line>`) and each claim's basis is allowed. Judge what they
cannot: whether the claims are true of the code, the guide, and the probes shown, useful, and honest about what is
unknown. Compare every claim you can with the files and guide lines the host read; a claim they contradict is a
fabrication, and so is a claim a cited guide line does not say. Judge this section only; the sections it rests on
are shown for context and were judged on their own.

Score 5 when every part of the criterion below holds, 4 when one part is weak, 3 when two are weak or one is
missing, 2 when the section would mislead a reader on any point, and 1 when it contains a fabrication (a claim the
cited code does not support, a user or customer the repository never names, or a number nothing measures).

1. **Guide and code are cross-checked.** The engineering claims explain the five algorithmic paths (a core
   subroutine and an easy-to-use driver each), the build, the test drivers, and the risks with their causes, and
   they read the guide against the code: where the guide says something this checkout no longer does, the draft says
   so with both citations (the guide's section 1.7 and dpmpar's comment describe DATA statements, while
   src/dpmpar.f computes the constants with epsilon, tiny, and huge; the guide describes a single precision version
   and SPMPAR that this checkout lacks). Each trap asks a question of this code and answers it from the code or a
   probe. A guide statement repeated as fact about this code without checking it, a trap listed without a finding,
   or a contradiction the files show that the draft misses, counts as weak.
