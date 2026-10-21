# Rubric: backlog drafted from a checked survey and its interview

The deterministic checks have passed: every item fits the work-item schema, rests on what the survey knows, and every
modernisation Story names its case sets. Judge what they cannot: whether this is a backlog a team could start on
tomorrow, and whether it does exactly what the person asked and nothing they ruled out.

Score 5 when all of these hold, 4 when one is weak, 3 when two are weak or one is missing, 2 when an item contradicts
an answer the person gave, and 1 when an item invents a requirement (work nobody asked for or the person ruled out,
such as fixing a quirk they kept or limiting scripts they called trusted).

1. **Faithful to the interview.** The scope follows the answers: only the three embedding calls the hosts use, every
   quirk kept for the first release, trusted scripts, the runner treated as a demo, and the deferred question left
   open rather than decided.
2. **Shape.** One Goal about the outcome for the people who rely on the software; Initiatives that separate
   characterisation from the rebuild; Features that group the work the way a team would deliver it; Stories small
   enough to finish and review, in dependency order (capture before port, values before parser); Bugs only for
   defects the survey found in the code.
3. **Executable Stories.** Each Completion Goal is observable and checkable by a command: parity runs name the case
   sets and expect no difference; other criteria name the state, the action, and a number or exact result. Measures
   and thresholds match the criteria.
4. **Parity is the spine.** Golden capture comes first, every port Story proves itself against the golden outputs,
   and the C++ build is retired only when every case set matches.
5. **Reviewable.** Titles say what changes in plain words; summaries say who benefits and why; nothing is padded,
   duplicated, or vague.
