# Rubric: traps, do-not-carry, and compatibility for a legacy Fortran library

The deterministic checks have passed: every citation resolves (including the probe facts), each claim's basis is
allowed, each trap names a rule, each do-not-carry entry has its evidence, risk, replacement, and an open deciding
question, and each interface's level is decided only by the request or an answered question. Judge what they cannot:
whether the findings are true of the code and the probes shown, specific to this codebase, and useful to a team about
to port it. Compare every claim you can with the files and probe output the host saw; a claim they contradict is a
fabrication.

Score 5 when all of these hold, 4 when one is weak, 3 when two are weak or one is missing, 2 when the draft would
mislead a reader on any point, and 1 when it contains a fabrication (a claim the cited code or probe does not
support, a caller or user the repository never names, a tolerance or performance number nobody gave, or a decision
the person has not made presented as made).

1. **Traps are checked, not listed.** Each finding asks a question of this code and answers it with what the code or a
   probe shows, including the ones a code audit cannot settle (that the Release flags change results, from the probe;
   that the COMMON block lives only in the drivers and why; whether implicit typing is actually relied on). A trap
   rule restated without a finding, or a generic Fortran warning that this code does not exhibit, counts as weak.
2. **Runtime evidence is used honestly.** It cites the probe facts for what running showed (results that change
   with the build, the routine the drivers never reach) and says what was not probed. It does not claim a cause
   (an instruction, a compiler pass) the probes do not show.
3. **The do-not-carry list is specific and decidable.** Each entry is a feature of this code a port would otherwise
   copy (state shared through COMMON, caller-allocated work arrays, duplicate test-problem names compiled into the
   library, fast-math as the release default, or others the code shows), with the risk of copying it, a concrete
   replacement, and the question the person decides it with; none is presented as already decided.
4. **The compatibility contract is right.** It names the interfaces callers depend on (the numbers the solvers
   return, the F77 names and argument lists, the interface module), sets a level only where the request says it
   ("the same results") and leaves the rest undecided with the question that decides, and its exact interfaces name
   exact evidence. It recognises that "the same results" needs a reference build because the probes show results
   vary with the build.
5. **The modernisation path proves parity.** Golden capture from a pinned legacy build comes first; steps name the
   case sets that prove them and list the do-not-carry entries in the code they replace; a shim keeps old callers
   working with a removal plan; shadow running compares both before cut-over.
