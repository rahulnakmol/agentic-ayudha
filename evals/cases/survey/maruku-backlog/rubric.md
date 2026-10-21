# Rubric: a compatibility backlog drafted from a checked survey and its interview

The deterministic checks have passed: every item fits the work-item schema and rests on what the survey knows, every
porting Story names its case sets or suites, the contract levels are respected, and every step's do-not-carry entries
are listed and constrained by a Decision. Judge what they cannot: whether a team could start on this backlog tomorrow,
whether it does exactly what the person asked and nothing they ruled out, and whether it would catch a page that
breaks.

Score 5 when all of these hold, 4 when one is weak, 3 when two are weak or one is missing, 2 when an item contradicts
an answer the person gave, and 1 when an item invents a requirement (work nobody asked for or the person ruled out,
such as supporting divs, math, or meta-data, keeping MaRuKu::Globals, porting the command, or byte-identical HTML).

1. **Faithful to the interview.** Scope follows the answers: equivalent HTML (Q1), only tables, footnotes, and header
   attribute lists (Q2), per-document fenced_code_blocks only (Q3), the command dropped with its deprecation path
   (Q4), and the Array patch, embedded Ruby, and global settings left behind as confirmed Decisions (Q3, Q5, Q6).
2. **Compatibility is the spine.** Maruku's own results are captured first, including the documents it fails itself;
   each Story that changes rendering names the contract it keeps (K1, K2, K3) and proves it with the suite and case
   set (S1 per area, G1); pages are shadow-rendered with both renderers before the switch; the switch has a tested
   way back; and the adapter (SH1) has a removal step.
3. **Shape.** One Goal about the outcome for the people who rely on the pages; Initiatives that separate recording
   today's behaviour from the replacement; Features grouped as a team would deliver them; Stories small enough to
   finish, in dependency order; Bugs only for defects that would hurt the pages before cut-over (a Bug to fix Maruku
   code the sites never reach is padding); Decisions that say what was decided, by whom, and why.
4. **Executable Stories.** Each Completion Goal is observable and checkable by a command (`--golden`, `--parity`,
   `--compat`, or a build), with a number or an exact result; measures and thresholds match the criteria, and none
   asks for a number nobody can produce.
5. **Reviewable.** Titles say what changes in plain words; summaries say who benefits and why; nothing is padded,
   duplicated, or vague.
