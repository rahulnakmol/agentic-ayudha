# Rubric: traps, do-not-carry, and a compatibility contract for a legacy Ruby library

The deterministic checks have passed: every citation resolves (including the probe facts), each claim's basis is
allowed, each trap names a rule, each do-not-carry entry has its evidence, risk, replacement, and an open deciding
question, and each interface's level is decided only by the request or an answered question. Judge what they cannot:
whether the findings are true of the code and the probes shown, specific to this codebase, and useful to a team
moving its pages to a maintained library. Compare every claim you can with the files and probe output the host saw;
a claim they contradict is a fabrication.

Score 5 when all of these hold, 4 when one is weak, 3 when two are weak or one is missing, 2 when the draft would
mislead a reader on any point, and 1 when it contains a fabrication (a claim the cited code or probe does not
support, a page, site, or caller the repository never names, a performance number nobody gave, or a decision the
person has not made presented as made).

1. **Traps are checked, not listed.** Each finding asks a question of this code and answers it with what the code or
   a probe shows: the class Maruku reopens for every program that loads it; that the code mutates string literals
   and, from the probe, that it fails under frozen string literals (where, and in how many of the cases run);
   process-wide settings; code evaluated from documents or options. A rule restated without a finding, or a generic
   Ruby warning this code does not exhibit, counts as weak.
2. **Runtime evidence is used honestly.** It cites the probe facts for what running showed and says what was not
   probed (for example, that only 20 block docs ran, or that no other Ruby version was tried). It does not claim a
   cause or a count the probes do not show.
3. **The do-not-carry list is specific and decidable.** Each entry is a feature of this code a replacement or adapter
   would otherwise copy (the Array patch, executing embedded Ruby, process-wide globals, or others the code shows),
   with the risk of copying it, a concrete replacement, and the question the person decides it with; none is
   presented as already decided.
4. **The compatibility contract is right.** It names what existing pages depend on (the HTML each page renders to,
   the Markdown dialect and its extensions, the options, the command if anything runs it) and leaves each level
   undecided with the question that decides it, since the request says pages must keep working but not how exactly.
   It does not assume a maintained library renders the same HTML: that is what the suite measures.
5. **The path proves compatibility.** Maruku's own block docs become the compatibility suite, scored per area,
   with the legacy results captured first (including the docs the legacy build itself fails); the replacement sits
   behind an adapter or shim with a removal plan; pages are compared side by side before cut-over; and steps avoid
   the do-not-carry entries in the code they touch.
