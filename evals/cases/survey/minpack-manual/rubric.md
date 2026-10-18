# Rubric: survey of a legacy Fortran library with its 1980 user guide

The deterministic checks have passed: every citation resolves (including the guide's lines, cited as
`manual:anl-80-74.pdf.txt:<line>`, and the probe facts) and each claim's basis is allowed. Judge what they cannot:
whether the claims are true of the code, the guide, and the probes shown, useful, and honest about what is unknown.
Compare every claim you can with the files and guide lines the host read; a claim they contradict is a fabrication,
and so is a claim a cited guide line does not say.

Score 5 when all of these hold, 4 when one is weak, 3 when two are weak or one is missing, 2 when the draft would
mislead a reader on any point, and 1 when it contains a fabrication (a claim the cited code does not support, a user
or customer the repository never names, or a number nothing measures).

1. **Guide and code are cross-checked.** The engineering claims explain the five algorithmic paths (a core
   subroutine and an easy-to-use driver each), the build, the test drivers, and the risks with their causes, and
   they read the guide against the code: where the guide says something this checkout no longer does, the draft says
   so with both citations (the guide's section 1.7 and dpmpar's comment describe DATA statements, while
   src/dpmpar.f computes the constants with epsilon, tiny, and huge; the guide describes a single precision version
   and SPMPAR that this checkout lacks). Each trap asks a question of this code and answers it from the code or a
   probe. A guide statement repeated as fact about this code without checking it, a trap listed without a finding,
   or a contradiction the files show that the draft misses, counts as weak.
2. **The capability map rests on code and guide.** Each capability is what a caller can do, in plain words, one per
   claim, cites the routine that does it, and cites the guide's description where the guide has one; a routine the
   guide does not document (covar) is said to be undocumented, and nothing the guide describes but the checkout
   lacks is claimed as a capability.
3. **Product claims come from the guide, labelled.** Who it is for and what jobs it serves are stated with the
   guide's lines (systems of nonlinear equations, nonlinear least squares, most users needing only the drivers), the
   guide's 1980 support statement is reported as history, not as current support, and what neither the code nor the
   guide says (the person's own callers, how exact the same results must be, who supports it now) is unknown with a
   question. No user, customer, or adoption figure is invented.
4. **The compatibility contract and the port prove parity.** The contract names what callers depend on (the
   numbers the solvers return, the F77 names and argument lists the guide documents, the interface module), sets a
   level only where the request says it ("the same results"), and leaves the rest undecided with its question; each
   do-not-carry entry is a feature of this code with its risk, a replacement, and an open question; golden capture from a pinned legacy build comes first, because the probes show results vary with the build, and
   each step names the case sets that prove it and the do-not-carry entries it replaces.
5. **Questions are the right ones.** Few, ordered by how much the answer changes the port: which callers and entry
   points, how exact the results must be, and whether the code or the guide is the reference where they disagree,
   each with why it matters and a default with a reason. No question is answered by the skill.
