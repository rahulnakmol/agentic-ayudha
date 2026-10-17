# Rubric: survey draft of a legacy C++ interpreter, with a Rust rebuild

The deterministic checks have passed: every citation resolves and each claim's basis is allowed. Judge what they
cannot: whether the claims are true of the code shown, useful, and honest about what is unknown. Compare every claim
you can with the files the host read; a claim the files contradict is a fabrication.

Score 5 when all of these hold, 4 when one is weak, 3 when two are weak or one is missing, 2 when the draft would
mislead a reader on any point, and 1 when it contains a fabrication (a claim the cited code does not support, a user
or customer the repository never names, or a number nothing measures).

1. **Engineering depth.** It explains the architecture (one interpreter class, recursive descent executing from
   source), the data model and memory management, the interfaces (the embedding API, the command-line runner in file
   and interactive modes, error output and exit codes), the build (Make and CMake targets), the test runner and how a
   test passes, dependencies, and the risks with their causes. It cross-checks what comments state against the code
   and reports any contradiction it can see (for example a known-issue line the version history or the code
   disagrees with), and notes differences between the runners (which built-ins each registers).
2. **Capability map.** Capabilities are what a host application or a person can do with the software, in plain
   words, one per claim, covering running scripts, evaluating for the host, native functions, variables, JSON, the
   built-in library, and the runner.
3. **Product level is honest.** Who it is for and what jobs it serves come only from the readme (stated) or are
   unknown with a question. It never names customers or users the repository does not name, and never claims
   adoption, market, or performance figures.
4. **Questions are the right ones.** Few, ordered by how much the answer changes the rebuild (which embedding API
   hosts use, whether quirks must be kept, untrusted scripts, whether the runner is part of the product), each with
   why it matters and a default with a reason. No question is answered by the skill.
5. **Modernisation path proves parity.** The target is Rust, from the request. Characterisation comes first, with
   golden outputs captured from the C++ build; each step names the case sets that prove it; case sets take inputs
   from the existing tests and the runner; non-deterministic behaviour (an unseeded random sequence) is excluded with
   a reason; risks name what makes byte-identical output hard in Rust (error positions, number formatting, memory
   model).
