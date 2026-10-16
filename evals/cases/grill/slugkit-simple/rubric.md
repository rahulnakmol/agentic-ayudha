# Rubric: grill conversation, "Make slugkit simple to use"

You see a whole grill conversation and the Story it produced. The request is vague on purpose: "simple" for a JavaScript library (`product:library`), a synthetic slug library whose only export is createSlugger(options) (`export:slugkit`), which throws unless all five options are given. Screens, commands, and endpoints do not apply. A scripted human answered each question by its
topic; each answer has a `kind`: `confirm`, `default` (accepted the skill's offered default), `park`, or `defer`.
The assessment's `gate` said `unpack`. Score three things, then give one overall score from 1 to 5.

1. Question quality: the questions are about a library: the calls and lines before a first result, which options get defaults and what they are, and what existing users rely on; they cite the export and test facts and come riskiest first.
2. Concreteness of the spec: every criterion names its state (a new project with only slugkit installed, the existing tests, the README) and an exact return value, a count, or the exact example; nothing still says "simple" or "easy". The readback says in plain language what done looks like: the goal, the
   criteria, and the checkpoints the human will see.
3. Fidelity: `slugify('Hello World')` and 'hello-world', the dash, lowercase, and no length limit, the unchanged createSlugger, and the README's first example are the human's; no other example values appear; renaming options (parked) is not a criterion. Test setup that a criterion's evidence needs (seeded data, a synthetic user, a fresh home directory) is not an added requirement; a state that narrows what the human asked for is. Questions and parked ideas never lower the score; only what became a criterion counts.

- 5: All three are strong.
- 4: One minor gap in one area.
- 3: Usable, but one area is weak, such as questions that ignore the repository or a criterion missing its state.
- 2: A detail the human did not give (a number, a place, a name), a parked or deferred topic as a criterion, or a
  criterion still vague.
- 1: Not a usable spec.
