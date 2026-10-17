# Rubric: grill conversation, a concrete request

You see a whole grill conversation and the Story it produced. The request is concrete on purpose: it names the screen (`/order/review`), the value, and the exact format ("£12.50"), and has no vague word or abbreviation. A scripted human answered each question by its
topic; each answer has a `kind`: `confirm`, `default` (accepted the skill's offered default), `park`, or `defer`.
The assessment's `gate` said `pass`. Score three things, then give one overall score from 1 to 5.

1. Question quality: the skill asks nothing, because the gate passed it: a concrete ask gets no friction. Any question scores this area low.
2. Concreteness of the spec: the one criterion names the state and the exact format from the request; Checkpoints and the readback say the same. The readback says in plain language what done looks like: the goal, the
   criteria, and the checkpoints the human will see.
3. Fidelity: the criterion is the request's own words (`stated (request)`); nothing is added that the request does not say, such as other screens, currencies, or rounding rules. A screenshot needs a state to be taken in: the seeded customer and the seeded basket are test setup, not an added requirement. A state that narrows the behaviour asked for (only when the basket is non-empty, only for some customers) is an addition. Questions and parked ideas never lower the score; only what became a criterion counts.

- 5: All three are strong.
- 4: One minor gap in one area.
- 3: Usable, but one area is weak, such as questions that ignore the repository or a criterion missing its state.
- 2: A detail the human did not give (a number, a place, a name), a parked or deferred topic as a criterion, or a
  criterion still vague.
- 1: Not a usable spec.
