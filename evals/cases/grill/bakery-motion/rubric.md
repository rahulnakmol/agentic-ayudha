# Rubric: grill conversation, "slick, with nice animations and transitions"

You see a whole grill conversation and the Story it produced. The request is vague on purpose: "slick" and "nice" are feelings, "animations and transitions" names no moment, duration, or proof, in a synthetic bakery app whose only motion is a 150 ms button fade (`motion:web/styles/buttons.css`) and which never honours reduced motion (`gap:reduced-motion`). A scripted human answered each question by its
topic; each answer has a `kind`: `confirm`, `default` (accepted the skill's offered default), `park`, or `defer`.
The assessment's `gate` said `unpack`. Score three things, then give one overall score from 1 to 5.

1. Question quality: the skill stops to unpack before anything else; the questions turn the motion into its moments, what moves, durations and easing, reduced motion, and proof; they cite the motion facts and come riskiest first, each with a default or options and a reason.
2. Concreteness of the spec: every criterion names its state, the moment, and a duration, count, or named result; reduced motion has its own criterion; nothing still says "slick", "nice", or "smooth". The readback says in plain language what done looks like: the goal, the
   criteria, and the checkpoints the human will see.
3. Fidelity: the durations (200 ms and 150 ms) and what moves are the human's; frame rate (parked) and the confirmation celebration (parked) are not criteria; easing is the accepted default, not invented. A criterion checks one representative instance (seeded data, a synthetic user, a fresh home directory, one seeded declining card for "a payment fails"): that is test setup, not an added requirement or a narrowing, as long as the expected result keeps the human's whole scope. It narrows only when its wording limits the behaviour itself to a subset the human did not choose ("only for guests", "only going forward"). Questions and parked ideas never lower the score; only what became a criterion counts.

- 5: All three are strong.
- 4: One minor gap in one area.
- 3: Usable, but one area is weak, such as questions that ignore the repository or a criterion missing its state.
- 2: A detail the human did not give (a number, a place, a name), a parked or deferred topic as a criterion, or a
  criterion still vague.
- 1: Not a usable spec.
