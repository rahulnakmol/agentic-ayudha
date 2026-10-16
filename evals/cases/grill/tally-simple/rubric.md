# Rubric: grill conversation, "Make the tally CLI simple to use"

You see a whole grill conversation and the Story it produced. The request is vague on purpose: "simple" for a command-line tool (`product:cli`), a synthetic expense tracker with 6 commands and 27 flags (`command:tally`) whose `tally add` needs three required flags and fails until `tally init` has run. A web app's questions (screens, tokens) do not apply. A scripted human answered each question by its
topic; each answer has a `kind`: `confirm`, `default` (accepted the skill's offered default), `park`, or `defer`.
The assessment's `gate` said `unpack`. Score three things, then give one overall score from 1 to 5.

1. Question quality: the questions are about a command-line tool: the first command a new user types, required flags and defaults, what an error says, exit codes, and help; they cite the command facts and come riskiest first.
2. Concreteness of the spec: every criterion names the command, its state (a fresh home directory), and an exit code, a count, or exact output; nothing still says "simple" or "easy". The readback says in plain language what done looks like: the goal, the
   criteria, and the checkpoints the human will see.
3. Fidelity: the first command, GBP, the plain-word amount and category, and the 24-line help are the human's; exit code 2 holds only where the human accepted that default (a `default` answer on exit codes); when the human was never asked about exit codes, a criterion that leaves the code open or deferred is not a gap; dropping report and export (parked) and remembered choices (parked) are not criteria. A criterion checks one representative instance (seeded data, a synthetic user, a fresh home directory, one seeded declining card for "a payment fails"): that is test setup, not an added requirement or a narrowing, as long as the expected result keeps the human's whole scope. It narrows only when its wording limits the behaviour itself to a subset the human did not choose ("only for guests", "only going forward"). Questions and parked ideas never lower the score; only what became a criterion counts.

- 5: All three are strong.
- 4: One minor gap in one area.
- 3: Usable, but one area is weak, such as questions that ignore the repository or a criterion missing its state.
- 2: A detail the human did not give (a number, a place, a name), a parked or deferred topic as a criterion, or a
  criterion still vague.
- 1: Not a usable spec.
