# Rubric: survey of a legacy C++ interpreter, with a Rust rebuild: the modernisation section

This section holds the do-not-carry list, compatibility contract, and modernisation path.

The deterministic checks have passed on this section and the sections before it: every citation resolves and
each claim's basis is allowed. Judge what they cannot: whether the claims are true of the code shown, useful, and
honest about what is unknown. Compare every claim you can with the files the host read; a claim the files
contradict is a fabrication. Judge this section only; the sections it rests on are shown for context and were
judged on their own.

Score 5 when every part of the criterion below holds, 4 when one part is weak, 3 when two are weak or one is
missing, 2 when the section would mislead a reader on any point, and 1 when it contains a fabrication (a claim the
cited code does not support, a user or customer the repository never names, or a number nothing measures).

1. **Modernisation path proves parity.** The target is Rust, from the request. Characterisation comes first, with
   golden outputs captured from the C++ build; each step names the case sets that prove it; case sets take inputs
   from the existing tests and the runner; non-deterministic behaviour (an unseeded random sequence) is excluded with
   a reason; risks name what makes byte-identical output hard in Rust (error positions, number formatting, memory
   model).
