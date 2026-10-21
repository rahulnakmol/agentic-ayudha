# Rubric: survey of a legacy C++ interpreter, with a Rust rebuild: the engineering section

This section holds the engineering and traps.

The deterministic checks have passed on this section and the sections before it: every citation resolves and
each claim's basis is allowed. Judge what they cannot: whether the claims are true of the code shown, useful, and
honest about what is unknown. Compare every claim you can with the files the host read; a claim the files
contradict is a fabrication. Judge this section only; the sections it rests on are shown for context and were
judged on their own.

Score 5 when every part of the criterion below holds, 4 when one part is weak, 3 when two are weak or one is
missing, 2 when the section would mislead a reader on any point, and 1 when it contains a fabrication (a claim the
cited code does not support, a user or customer the repository never names, or a number nothing measures).

1. **Engineering depth.** It explains the architecture (one interpreter class, recursive descent executing from
   source), the data model and memory management, the interfaces (the embedding API, the command-line runner in file
   and interactive modes, error output and exit codes), the build (Make and CMake targets), the test runner and how a
   test passes, dependencies, and the risks with their causes. It cross-checks what comments state against the code
   and reports any contradiction it can see (for example a known-issue line the version history or the code
   disagrees with), and notes differences between the runners (which built-ins each registers).
