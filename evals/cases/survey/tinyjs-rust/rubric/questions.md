# Rubric: survey of a legacy C++ interpreter, with a Rust rebuild: the questions section

This section holds the questions and summary.

The deterministic checks have passed on this section and the sections before it: every citation resolves and
each claim's basis is allowed. Judge what they cannot: whether the claims are true of the code shown, useful, and
honest about what is unknown. Compare every claim you can with the files the host read; a claim the files
contradict is a fabrication. Judge this section only; the sections it rests on are shown for context and were
judged on their own.

Score 5 when every part of the criterion below holds, 4 when one part is weak, 3 when two are weak or one is
missing, 2 when the section would mislead a reader on any point, and 1 when it contains a fabrication (a claim the
cited code does not support, a user or customer the repository never names, or a number nothing measures).

1. **Questions are the right ones.** Few, ordered by how much the answer changes the rebuild (which embedding API
   hosts use, whether quirks must be kept, untrusted scripts, whether the runner is part of the product), each with
   why it matters and a default with a reason. No question is answered by the skill.
